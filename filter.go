package query

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

type Position struct {
	Line int
	Col  int
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

func Filter(r io.Reader, query string) (string, error) {
	q, err := Parse(query)
	if err != nil {
		return "", err
	}
	err = Execute(r, q)
	if err != nil {
		return "", err
	}
	return q.Get(), nil
}

func Execute(r io.Reader, q Query) error {
	rs := prepare(r)
	return rs.Read(q)
}

var errDone = errors.New("done")

type reader struct {
	inner io.RuneScanner
	file  string
	depth int

	prev Position
	curr Position
}

func prepare(r io.Reader) *reader {
	rs := reader{
		inner: bufio.NewReader(r),
		file:  "<input>",
	}
	rs.curr.Line = 1
	if n, ok := r.(interface{ Name() string }); ok {
		rs.file = n.Name()
	}
	return &rs
}

func (r *reader) Read(q Query) error {
	c, err := r.read()
	if err != nil {
		return err
	}
	switch {
	case jsonQuote(c):
		_, err = r.literal()
	case jsonIdent(c):
		_, err = r.identifier()
	case jsonDigit(c):
		_, err = r.number()
	case jsonArray(c):
		err = r.array(q)
	case jsonObject(c):
		err = r.object(q)
	default:
		err = r.malformed("unexpected character %c", c)
	}
	return err
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
		if err = r.traverse(q, key); err != nil {
			return err
		}
		if err := r.endObject(); err != nil {
			if errors.Is(err, errDone) {
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

func (r *reader) array(q Query) error {
	if err := canArray(q); err != nil {
		return err
	}
	for i := 0; ; i++ {
		err := r.traverse(q, strconv.Itoa(i))
		if err != nil {
			return err
		}
		if err := r.endArray(); err != nil {
			if errors.Is(err, errDone) {
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

func (r *reader) traverse(q Query, key string) error {
	next, err := q.Next(key)
	if err != nil {
		return r.Read(KeepAll)
	}
	var wrapped bool
	if wrapped = next == nil; wrapped {
		r.wrap()
		next = KeepAll
	}
	if err = r.Read(next); err != nil {
		return err
	}
	if wrapped {
		str := r.unwrap()
		if s, ok := q.(setter); ok {
			s.set(str)
		}
	}
	return nil
}

func (r *reader) literal() (string, error) {
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
	case 'n', 'f', 'b', 'r', '"', '\\':
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

func (r *reader) identifier() (string, error) {
	defer r.unread()
	r.unread()

	var buf bytes.Buffer
	for {
		c, err := r.read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
		if !jsonLetter(c) {
			break
		}
		buf.WriteRune(c)
	}
	switch ident := buf.String(); ident {
	case "true", "false", "null":
		return ident, nil
	default:
		return "", r.malformed("%s: identifier not recognized", ident)
	}
}

func (r *reader) number() (string, error) {
	var (
		buf bytes.Buffer
		err error
	)
	r.unread()
	if c, _ := r.read(); c == '0' {
		buf.WriteRune(c)
		if c, _ = r.read(); c == '.' {
			err := r.fraction(&buf)
			return buf.String(), err
		} else if jsonBlank(c) || c == ',' || c == '}' || c == ']' {
			r.unread()
			return buf.String(), nil
		}
		return "", r.malformed("expected fraction after 0")
	}
	r.unread()
	for {
		c, _ := r.read()
		if !jsonDigit(c) {
			break
		}
		buf.WriteRune(c)
	}
	r.unread()
	switch c, _ := r.read(); c {
	case '.':
		err = r.fraction(&buf)
	case 'e', 'E':
		err = r.exponent(&buf, c)
	default:
		r.unread()
	}
	return buf.String(), err
}

func (r *reader) fraction(buf *bytes.Buffer) error {
	if c, _ := r.read(); !jsonDigit(c) {
		return r.malformed("expected digit after '.'")
	}
	r.unread()

	defer r.unread()
	buf.WriteRune('.')
	for {
		c, _ := r.read()
		if !jsonDigit(c) {
			break
		}
		buf.WriteRune(c)
	}
	r.unread()
	if c, _ := r.read(); c == 'e' || c == 'E' {
		return r.exponent(buf, c)
	}
	return nil
}

func (r *reader) exponent(buf *bytes.Buffer, exp rune) error {
	defer r.unread()

	buf.WriteRune(exp)
	c, _ := r.read()
	if c == '-' || c == '+' {
		buf.WriteRune(c)
		c, _ = r.read()
	}
	if !jsonDigit(c) || c == '0' {
		return r.malformed("expected digit (different of 0) after exponent")
	}
	buf.WriteRune(c)
	for {
		c, _ := r.read()
		if !jsonDigit(c) {
			break
		}
		buf.WriteRune(c)
	}
	return nil
}

func (r *reader) enter() {
	r.depth++
}

func (r *reader) leave() {
	r.depth--
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
		if !jsonBlank(c) {
			return c, err
		}
	}
}

func (r *reader) unread() {
	r.inner.UnreadRune()
}

func (r *reader) wrap() {
	r.inner = wrap(r.inner)
}

func (r *reader) unwrap() string {
	w, ok := r.inner.(*writer)
	if ok {
		r.inner = w.Unwrap()
		return w.String()
	}
	return ""
}

func (r *reader) malformed(msg string, args ...interface{}) error {
	return MalformedError{
		Position: r.curr,
		File:     r.file,
		Message:  fmt.Sprintf(msg, args...),
	}
}

func canObject(q Query) error {
	switch q.(type) {
	case *all, *ident, *any, *object, *array:
		return nil
	default:
		return invalidQueryForType("object")
	}
}

func canArray(q Query) error {
	switch q.(type) {
	case *all, *index:
		return nil
	default:
		return invalidQueryForType("array")
	}
}

type writer struct {
	io.RuneScanner
	buf bytes.Buffer
}

func wrap(rs io.RuneScanner) io.RuneScanner {
	if _, ok := rs.(*writer); ok {
		return rs
	}
	return &writer{
		RuneScanner: rs,
	}
}

func (w *writer) ReadRune() (rune, int, error) {
	c, z, err := w.RuneScanner.ReadRune()
	if err == nil && !jsonBlank(c) {
		w.buf.WriteRune(c)
	}
	return c, z, err
}

func (w *writer) UnreadRune() error {
	err := w.RuneScanner.UnreadRune()
	if err == nil {
		w.buf.Truncate(w.buf.Len() - 1)
	}
	return err
}

func (w *writer) Unwrap() io.RuneScanner {
	return w.RuneScanner
}

func (w *writer) String() string {
	return w.buf.String()
}

func jsonBlank(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

func jsonQuote(r rune) bool {
	return r == '"'
}

func jsonArray(r rune) bool {
	return r == '['
}

func jsonObject(r rune) bool {
	return r == '{'
}

func jsonDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func jsonIdent(r rune) bool {
	return r == 't' || r == 'f' || r == 'n'
}

func jsonLetter(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func jsonHex(r rune) bool {
	return jsonDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}
