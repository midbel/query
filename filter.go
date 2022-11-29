package query

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

func Filter(r io.Reader, query string) ([]string, error) {
	q, err := Parse(query)
	if err != nil {
		return nil, err
	}
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

func (r *reader) Read(q Query) ([]string, error) {
	c, err := r.read()
	if err != nil {
		return nil, err
	}
	switch {
	case jsonQuote(c):
		_, err := r.literal()
		return nil, err
	case jsonIdent(c):
		_, err := r.identifier()
		return nil, err
	case jsonDigit(c):
		return nil, r.number()
	case jsonArray(c):
		return r.array(q)
	case jsonObject(c):
		return r.object(q)
	default:
		return nil, fmt.Errorf("unexpected character %c", c)
	}
}

func (r *reader) object(q Query) ([]string, error) {
	r.enter()
	defer r.leave()

	var rs []string
	for {
		key, err := r.key()
		if err != nil {
			return nil, err
		}
		ns, err := r.traverse(q, key)
		if err != nil {
			return nil, err
		}
		rs = append(rs, ns...)
		if err := r.endObject(); err != nil {
			if errors.Is(err, errDone) {
				break
			}
			return nil, err
		}
	}
	return rs, nil
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

func (r *reader) array(q Query) ([]string, error) {
	var rs []string
	for i := 0; ; i++ {
		ns, err := r.traverse(q, strconv.Itoa(i))
		if err != nil {
			return nil, err
		}
		rs = append(rs, ns...)

		if err := r.endArray(); err != nil {
			if errors.Is(err, errDone) {
				break
			}
			return nil, err
		}
	}
	return rs, nil
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

func (r *reader) traverse(q Query, key string) ([]string, error) {
	next, err := q.Next(key)
	if err != nil {
		_, err = r.Read(KeepAll)
		return nil, err
	}
	var wrapped bool
	if wrapped = next == nil; wrapped {
		r.wrap()
		next = KeepAll
	}
	ns, err := r.Read(next)
	if err != nil {
		return nil, err
	}
	if wrapped {
		str := r.unwrap()
		if str != "" {
			ns = append(ns, str)
		}
	}
	return ns, nil
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

func (r *reader) number() error {
	return nil
}

func (r *reader) enter() {
	r.depth++
}

func (r *reader) leave() {
	r.depth--
}

func (r *reader) read() (rune, error) {
	c, _, err := r.inner.ReadRune()
	if err == nil && jsonBlank(c) {
		return r.read()
	}
	return c, err
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
