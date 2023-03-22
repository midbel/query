package query

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
)

func Execute(r io.Reader, query string) (string, error) {
	q, err := Parse(query)
	if err != nil {
		return "", err
	}
	return execute(r, q)
}

func execute(r io.Reader, q Query) (string, error) {
	var (
		pr, pw = io.Pipe()
		errch  = make(chan error, 1)
		strch  = make(chan string, 1)
		wg     sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		var res bytes.Buffer
		io.Copy(&res, pr)
		strch <- res.String()
	}()

	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			pr.Close()
			pw.Close()
		}()
		errch <- readFrom(r, pw).Read(q)
	}()
	wg.Wait()
	return <-strch, <-errch
}

type Position struct {
	Line int
	Col  int
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

type reader struct {
	inner  io.RuneScanner
	writer *writer

	file  string
	depth int

	prev      Position
	curr      Position
	keepBlank bool
}

func readFrom(r io.Reader, w io.Writer) *reader {
	rs := reader{
		inner:  bufio.NewReader(r),
		file:   "<input>",
		writer: writeTo(w),
	}
	rs.curr.Line = 1
	if n, ok := r.(interface{ Name() string }); ok {
		rs.file = n.Name()
	}
	return &rs
}

func (r *reader) Read(q Query) error {
	defer r.writer.close()
	if keepAll(q) {
		r.writer.toggle()
		defer r.writer.toggle()
	}
	err := r.traverse(q)
	if err != nil {
		return err
	}
	if _, err = r.read(); err == nil {
		return r.malformed("malformed JSON document: unexpected end")
	}
	return nil
}

func (r *reader) traverse(q Query) error {
	c, err := r.read()
	if err != nil {
		return err
	}
	switch {
	case jsonQuote(c):
		_, err = r.literal()
	case jsonIdent(c):
		err = r.identifier()
	case jsonDigit(c):
		err = r.number()
	case jsonArray(c):
		err = r.array(q)
	case jsonObject(c):
		err = r.object(q)
	default:
		err = r.malformed("unexpected character %c", c)
	}
	return err
}

func (r *reader) literal() (string, error) {
	r.toggleBlank()
	defer r.toggleBlank()

	var buf bytes.Buffer
	for {
		c, err := r.read()
		if err != nil {
			return "", err
		}
		if jsonQuote(c) {
			break
		}
		if c == '\\' {
			if err := r.escape(&buf); err != nil {
				return "", err
			}
			continue
		}
		buf.WriteRune(c)
	}
	return buf.String(), nil
}

func (r *reader) escape(buf *bytes.Buffer) error {
	buf.WriteRune('\\')
	switch c, _ := r.read(); c {
	case 'n', 'f', 'b', 'r', '"', '\\', '/':
		buf.WriteRune(c)
	case 'u':
		buf.WriteRune(c)
		for i := 0; i < 4; i++ {
			c, _ = r.read()
			if !jsonHex(c) {
				return r.malformed("%c not a hex character", c)
			}
			buf.WriteRune(c)
		}
	default:
		return r.malformed("unknown escape \\%c", c)
	}
	return nil
}

func (r *reader) identifier() error {
	defer r.unread()
	r.unread()

	var buf bytes.Buffer
	for {
		c, err := r.read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if !jsonLetter(c) {
			break
		}
		buf.WriteRune(c)
	}
	switch ident := buf.String(); ident {
	case "true":
	case "false":
	case "null":
	default:
		return r.malformed("%s: identifier not recognized", ident)
	}
	return nil
}

func (r *reader) number() error {
	var err error
	r.unread()
	if c, _ := r.read(); c == '0' {
		if c, _ = r.read(); c == '.' {
			return r.fraction()
		} else if jsonBlank(c) || c == ',' || c == '}' || c == ']' {
			r.unread()
			return nil
		}
		return r.malformed("expected fraction after 0")
	}
	r.unread()
	for {
		c, _ := r.read()
		if !jsonDigit(c) {
			break
		}
	}
	r.unread()
	switch c, _ := r.read(); c {
	case '.':
		err = r.fraction()
	case 'e', 'E':
		err = r.exponent()
	default:
		r.unread()
	}
	return err
}

func (r *reader) fraction() error {
	if c, _ := r.read(); !jsonDigit(c) {
		return r.malformed("expected digit after '.'")
	}
	r.unread()

	defer r.unread()
	for {
		c, _ := r.read()
		if !jsonDigit(c) {
			break
		}
	}
	r.unread()
	if c, _ := r.read(); c == 'e' || c == 'E' {
		return r.exponent()
	}
	return nil
}

func (r *reader) exponent() error {
	defer r.unread()

	c, _ := r.read()
	if c == '-' || c == '+' {
		c, _ = r.read()
	}
	if !jsonDigit(c) || c == '0' {
		return r.malformed("expected digit (different of 0) after exponent")
	}
	for {
		c, _ := r.read()
		if !jsonDigit(c) {
			break
		}
	}
	return nil
}

func (r *reader) key() (string, error) {
	c, _ := r.read()
	if !jsonQuote(c) {
		return "", r.malformed("key: expected '\"' instead of %c", c)
	}
	key, err := r.literal()
	if err != nil {
		return "", err
	}
	if c, _ = r.read(); c != ':' {
		return "", r.malformed("key: expected ':' instead of %c", c)
	}
	return key, nil
}

func (r *reader) object(q Query) error {
	if err := canObject(q); err != nil {
		return err
	}
	r.enter()
	defer r.leave()

	for {
		key, err := r.key()
		if err != nil {
			return err
		}
		if err = r.filter(q, key); err != nil {
			return err
		}
		if err := r.endObject(); err != nil {
			if isDone(err) {
				break
			}
			return err
		}
	}
	return nil
}

func (r *reader) endObject() error {
	if c, _ := r.read(); c == '}' {
		return errDone
	} else if c == ',' {
		if c, err := r.read(); c == '}' || err != nil {
			return r.malformed("object: unexpected character after ','")
		}
		r.unread()
	} else {
		return r.malformed("object: expected ',' or '}'")
	}
	return nil
}

func (r *reader) array(q Query) error {
	r.enter()
	defer r.leave()

	if err := canArray(q); err != nil {
		return err
	}
	for i := 0; ; i++ {
		err := r.filter(q, strconv.Itoa(i))
		if err != nil {
			return err
		}
		if err := r.endArray(); err != nil {
			if isDone(err) {
				break
			}
			return err
		}
	}
	return nil
}

func (r *reader) endArray() error {
	if c, _ := r.read(); c == ']' {
		return errDone
	} else if c == ',' {
		if c, err := r.read(); c == ']' || err != nil {
			return r.malformed("array: unexpected character after ','")
		}
		r.unread()
	} else {
		return r.malformed("array: expected ',' or ']")
	}
	return nil
}

func (r *reader) filter(q Query, key string) error {
	if q == nil {
		return r.traverse(q)
	}
	next, err := q.Next(key)
	if err != nil {
		return r.traverse(next)
	}
	if !keepAll(q) && next == nil {
		r.writer.toggle()
		defer r.writer.toggle()
	}
	return r.traverse(next)
}

func (r *reader) toggleBlank() {
	r.keepBlank = !r.keepBlank
}

func (r *reader) enter() {
	r.depth++
}

func (r *reader) leave() {
	r.depth--
}

func (r *reader) unread() {
	r.inner.UnreadRune()
	r.writer.unwriteRune()
	r.curr = r.prev
}

func (r *reader) read() (rune, error) {
	for {
		c, _, err := r.inner.ReadRune()
		r.prev = r.curr
		if c == '\n' {
			r.curr.Line++
			r.curr.Col = 0
		}
		r.curr.Col++
		if r.keepBlank || !jsonBlank(c) {
			r.writer.writeRune(c)
			return c, err
		}
	}
}

func (r *reader) malformed(msg string, args ...interface{}) error {
	return MalformedError{
		Position: r.curr,
		File:     r.file,
		Message:  fmt.Sprintf(msg, args...),
	}
}

var errDone = errors.New("done")

func isDone(err error) bool {
	return errors.Is(err, errDone)
}

func canObject(q Query) error {
	return nil
}

func canArray(q Query) error {
	return nil
}
