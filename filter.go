package query

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

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
	depth int
}

func prepare(r io.Reader) *reader {
	return &reader{
		inner: bufio.NewReader(r),
	}
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
		err = fmt.Errorf("unexpected character %c", c)
	}
	return err
}

func (r *reader) object(q Query) error {
	r.enter()
	defer r.leave()

	var seen = make(map[string]struct{})
	for {
		key, err := r.key()
		if err != nil {
			return err
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("object: duplicate key %q", key)
		}
		seen[key] = struct{}{}

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
			return fmt.Errorf("object: unexpected character after ','")
		}
		r.unread()
	} else {
		return fmt.Errorf("object: expected ',' or '}'")
	}
	return nil
}

func (r *reader) key() (string, error) {
	c, _ := r.read()
	if !jsonQuote(c) {
		return "", fmt.Errorf("key: expected '\"' instead of %c", c)
	}
	key, err := r.literal()
	if err != nil {
		return "", err
	}
	if c, _ = r.read(); c != ':' {
		return "", fmt.Errorf("key: expected ':' instead of %c", c)
	}
	return key, nil
}

func (r *reader) array(q Query) error {
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
			return fmt.Errorf("array: unexpected character after ','")
		}
		r.unread()
	} else {
		return fmt.Errorf("array: expected ',' or ']")
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
				return fmt.Errorf("%c not a hex character", c)
			}
			buf.WriteRune(c)
		}
	default:
		return fmt.Errorf("unknown escape \\%c", c)
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
		return "", fmt.Errorf("%s: identifier not recognized", ident)
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
		}
		return "", fmt.Errorf("expected fraction after 0")
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
		return fmt.Errorf("expected digit after '.'")
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
		return fmt.Errorf("expected digit (different of 0) after exponent")
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
