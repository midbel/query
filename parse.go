package query

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrSkip = errors.New("skip")

var KeepAll Filter = all{}

type Filter interface {
	Next(string) (Filter, error)
}

// ident[.query][,query]
func Parse(str string) (Filter, error) {
	if str == "." {
		return KeepAll, nil
	}
	r := strings.NewReader(str)
	return parse(r)
}

func parse(r io.RuneScanner) (Filter, error) {
	var list []Filter
	for {
		c, _, err := r.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		switch c {
		case '.':
		case ',':
			c, _, _ = r.ReadRune()
		default:
			return nil, fmt.Errorf("unexpected character %c", c)
		}
		if c != '.' {
			return nil, fmt.Errorf("missing '.' before identifier")
		}
		var i ident
		if err := parseQuery(r, &i); err != nil {
			return nil, err
		}
		list = append(list, &i)
	}
	if len(list) == 1 {
		return list[0], nil
	}
	a := any{
		list: list,
	}
	return &a, nil
}

func parseIdent(r io.RuneScanner, q *ident) error {
	r.UnreadRune()
	var str bytes.Buffer
	for {
		c, _, err := r.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if !isAlpha(c) {
			r.UnreadRune()
			break
		}
		str.WriteRune(c)
	}
	q.ident = str.String()
	return nil
}

func parseQuote(r io.RuneScanner, q *ident, quote rune) error {
	var str bytes.Buffer
	for {
		c, _, err := r.ReadRune()
		if err != nil {
			return err
		}
		if c == quote {
			break
		}
		str.WriteRune(c)
	}
	q.ident = str.String()
	return nil
}

func parseQuery(r io.RuneScanner, q *ident) error {
	var err error
	c, _, _ := r.ReadRune()
	if isQuote(c) {
		err = parseQuote(r, q, c)
	} else if isLetter(c) {
		err = parseIdent(r, q)
	} else {
		err = fmt.Errorf("identifier: unexpected character %c", c)
	}
	if err != nil {
		return err
	}
	switch c, _, tmp := r.ReadRune(); c {
	case '.':
		r.UnreadRune()
		q.next, err = parse(r)
	case ',':
		r.UnreadRune()
	default:
		if errors.Is(tmp, io.EOF) {
			return nil
		}
		err = fmt.Errorf("identifier: unexpected character %c", c)
	}
	return err
}

func parseArray(r io.RuneScanner) error {
	for {
		parseInt(r)
		c, _, err := r.ReadRune()
		if err != nil {
			return err
		}
		switch c {
		case ',':
		case ']':
			return nil
		default:
			return fmt.Errorf("unexpected character %c", c)
		}
	}
	return nil
}

func parseInt(r io.RuneScanner) {
	defer r.UnreadRune()
	for {
		c, _, _ := r.ReadRune()
		if !isDigit(c) {
			break
		}
	}
}

func isAlpha(r rune) bool {
	return isLower(r) || isUpper(r) || isDigit(r) || r == '_'
}

func isLetter(r rune) bool {
	return isLower(r) || isUpper(r)
}

func isLower(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isBlank(r rune) bool {
	return r == ' ' || r == '\t'
}

func isQuote(r rune) bool {
	return r == '\'' || r == '"'
}
