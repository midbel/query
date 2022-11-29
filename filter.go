package query

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

func Filter(r io.Reader, query string) (interface{}, error) {
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

func (r *reader) Read(q Query) (interface{}, error) {
	c, err := r.read()
	if err != nil {
		return nil, err
	}
	switch {
	case jsonQuote(c):
		return r.literal()
	case jsonIdent(c):
		return r.identifier()
	case jsonDigit(c):
		return r.number()
	case jsonArray(c):
		return r.array(q)
	case jsonObject(c):
		return r.object(q)
	default:
		return nil, fmt.Errorf("unexpected character %c", c)
	}
}

func (r *reader) object(q Query) (interface{}, error) {
	r.enter()
	defer r.leave()

	var (
		obj = make(map[string]interface{})
		arr []interface{}
	)
	for {
		key, err := r.key()
		if err != nil {
			return nil, err
		}
		v, err := r.traverse(q, key)
		if err != nil {
			return nil, err
		}
		if v != nil {
			obj[key] = v
			arr = append(arr, v)
		}
		if err := r.endObject(); err != nil {
			if errors.Is(err, errDone) {
				break
			}
			return nil, err
		}
	}
	if q == KeepAll {
		return obj, nil
	}
	return firstOrAll(arr), nil
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

func (r *reader) array(q Query) (interface{}, error) {
	var arr []interface{}
	for i := 0; ; i++ {
		v, err := r.traverse(q, strconv.Itoa(i))
		if err != nil {
			return nil, err
		}
		if v != nil {
			arr = append(arr, v)
		}

		if err := r.endArray(); err != nil {
			if errors.Is(err, errDone) {
				break
			}
			return nil, err
		}
	}
	return firstOrAll(arr), nil
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

func (r *reader) traverse(q Query, key string) (interface{}, error) {
	next, err := q.Next(key)
	if err != nil {
		_, err = r.Read(KeepAll)
		return nil, err
	}
	if next == nil {
		next = KeepAll
	}
	return r.Read(next)
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
		return nil, fmt.Errorf("%s: identifier not recognized", ident)
	}
}

func (r *reader) number() (float64, error) {
	var (
		buf bytes.Buffer
		err error
	)
	r.unread()
	if c, _ := r.read(); c == '0' {
		buf.WriteRune(c)
		if c, _ = r.read(); c == '.' {
			err := r.fraction(&buf)
			if err != nil {
				return 0, err
			}
			return getFloat(buf.String())
		}
		return 0, fmt.Errorf("expected fraction after 0")
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
		return 0, err
	}
	return getFloat(buf.String())
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
	c, _, err := r.inner.ReadRune()
	if err == nil && jsonBlank(c) {
		return r.read()
	}
	return c, err
}

func (r *reader) unread() {
	r.inner.UnreadRune()
}

func firstOrAll(arr []interface{}) interface{} {
	if len(arr) == 1 {
		return arr[0]
	}
	return arr
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
