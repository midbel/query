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

func Filter(r io.Reader, query string) ([]string, error) {
	q, err := Parse(query)
	if err != nil {
		return nil, err
	}
	if err := execute(r, q); err != nil {
		return nil, err
	}
	return q.Get(), nil
}

func Execute(r io.Reader, query string) (string, error) {
	q, err := Parse(query)
	if err != nil {
		return "", err
	}
	if err := execute(r, q); err != nil {
		return "", err
	}
	return q.String(), nil
}

func execute(r io.Reader, q Query) error {
	rs := prepare(r)
	return rs.Read(q)
}

type reader struct {
	inner io.RuneScanner
	file  string
	depth int

	prev      Position
	curr      Position
	keepBlank bool
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
	if keepAll(q) {
		r.wrap()
		defer r.update(q, "")
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
		r.wrap()
		defer r.update(q, key)
	}
	return r.traverse(next)
}

func (r *reader) update(q Query, key string) error {
	str := r.unwrap()
	return q.update(str)
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

func (r *reader) toggleBlank() {
	r.keepBlank = !r.keepBlank
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

func (r *reader) identifier() (interface{}, error) {
	defer r.unread()
	r.unread()

	var buf bytes.Buffer
	for {
		c, err := r.read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if !jsonLetter(c) {
			break
		}
		buf.WriteRune(c)
	}
	switch ident := buf.String(); ident {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
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
	if err != nil {
		return "", err
	}
	return buf.String(), nil
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
		if r.keepBlank || !jsonBlank(c) {
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
	w, ok := r.inner.(unwrapper)
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

var errDone = errors.New("done")

func isDone(err error) bool {
	return errors.Is(err, errDone)
}

func canObject(q Query) error {
	// if q == nil {
	// 	return nil
	// }
	// switch q.(type) {
	// case *all, *ident, *any, *object, *array:
	// 	return nil
	// default:
	// 	return invalidQueryForType("object")
	// }
	return nil
}

func canArray(q Query) error {
	// if q == nil {
	// 	return nil
	// }
	// switch q.(type) {
	// case *all, *index, *array:
	// 	return nil
	// default:
	// 	return invalidQueryForType("array")
	// }
	return nil
}

type unwrapper interface {
	fmt.Stringer
	Unwrap() io.RuneScanner
}

type compact struct {
	io.RuneScanner

	last    rune
	scanstr bool
	buf     bytes.Buffer
}

func wrap(rs io.RuneScanner) io.RuneScanner {
	if _, ok := rs.(*compact); ok {
		return rs
	}
	return &compact{
		RuneScanner: rs,
	}
}

func (w *compact) String() string {
	return w.buf.String()
}

func (w *compact) ReadRune() (rune, int, error) {
	c, z, err := w.RuneScanner.ReadRune()
	w.toggle(c)
	if err == nil && w.keep(c) {
		w.buf.WriteRune(c)
		if !w.scanstr && jsonSep(c) {
			w.buf.WriteRune(' ')
			w.last = c
		}
	}
	return c, z, err
}

func (w *compact) UnreadRune() error {
	err := w.RuneScanner.UnreadRune()
	if err == nil && w.buf.Len() > 0 {
		var size int
		if !w.scanstr && jsonSep(w.last) {
			size++
		}
		size++
		w.buf.Truncate(w.buf.Len() - size)
		w.toggle(w.last)
	}
	return err
}

func (w *compact) Unwrap() io.RuneScanner {
	return w.RuneScanner
}

func (w *compact) keep(c rune) bool {
	return !jsonBlank(c) || w.scanstr
}

func (w *compact) toggle(c rune) {
	if c == '"' && w.last != '\\' {
		w.scanstr = !w.scanstr
	}
	w.last = c
}

func jsonSep(r rune) bool {
	return r == ',' || r == ':'
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
