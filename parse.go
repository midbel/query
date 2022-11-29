package query

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
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
			if len(list) == 0 {
				return nil, fmt.Errorf("',' not valid at this position")
			}
			c, _, _ = r.ReadRune()
		default:
			return nil, fmt.Errorf("unexpected character %c", c)
		}
		if c != '.' {
			return nil, fmt.Errorf("expected '.' before identifier")
		}
		q, err := parseQuery(r)
		if err != nil {
			return nil, err
		}
		list = append(list, q)
	}
	if len(list) == 1 {
		return list[0], nil
	}

	a := any{
		list: list,
	}
	return &a, nil
}

func parseQuery(r io.RuneScanner) (Filter, error) {
	var (
		err   error
		query Filter
	)
	switch c, _, _ := r.ReadRune(); {
	case isQuote(c):
		query, err = parseIdent(r, quoteIdent)
	case isLetter(c):
		query, err = parseIdent(r, literalIdent)
	case isArray(c):
		query, err = parseArray(r)
	case isGroup(c):
		query, err = parseGroup(r)
	default:
		err = fmt.Errorf("query: unexpected character %c", c)
	}
	return query, err
}

func literalIdent(r io.RuneScanner) (string, error) {
	var str bytes.Buffer
	for {
		c, _, err := r.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
		if !isAlpha(c) {
			r.UnreadRune()
			break
		}
		str.WriteRune(c)
	}
	return str.String(), nil
}

func quoteIdent(r io.RuneScanner) (string, error) {
	quote, _, _ := r.ReadRune()
	var str bytes.Buffer
	for {
		c, _, err := r.ReadRune()
		if err != nil {
			return "", err
		}
		if c == quote {
			break
		}
		str.WriteRune(c)
	}
	return str.String(), nil
}

func parseIdent(r io.RuneScanner, get func(io.RuneScanner) (string, error)) (Filter, error) {
	r.UnreadRune()

	var (
		query ident
		err   error
	)
	query.ident, err = get(r)
	if err != nil {
		return nil, err
	}
	switch c, _, tmp := r.ReadRune(); c {
	case '.':
		query.next, err = parseQuery(r)
	case '[':
		query.next, err = parseArray(r)
	case ',', ')':
		r.UnreadRune()
	default:
		if errors.Is(tmp, io.EOF) {
			return &query, nil
		}
		err = fmt.Errorf("identifier: unexpected character %c", c)
	}
	return &query, err
}

func parseGroup(r io.RuneScanner) (Filter, error) {
	var grp group
	for {
		f, err := parseQuery(r)
		if err != nil {
			return nil, err
		}
		grp.list = append(grp.list, f)
		switch c, _, _ := r.ReadRune(); c {
		case ',':
		case ')':
			return &grp, nil
		default:
			return nil, fmt.Errorf("array: unexpected character %c", c)
		}
	}
	return &grp, nil
}

func parseArray(r io.RuneScanner) (Filter, error) {
	var arr array
	skipBlank(r)
	if c, _, _ := r.ReadRune(); c == ']' {
		return &arr, nil
	}
	r.UnreadRune()
	for {
		skipBlank(r)
		n, err := parseInt(r)
		if err != nil {
			return nil, err
		}
		arr.index = append(arr.index, n)

		switch c, _, _ := r.ReadRune(); c {
		case ',':
			skipBlank(r)
		case ']':
			c, _, err := r.ReadRune()
			if c == '.' {
				arr.next, err = parseQuery(r)
			}
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return &arr, err
		default:
			return nil, fmt.Errorf("array: unexpected character %c", c)
		}
	}
	return &arr, nil
}

func parseInt(r io.RuneScanner) (string, error) {
	defer r.UnreadRune()


	var str bytes.Buffer
	c, _, _ := r.ReadRune()
	if c == '-' {
		str.WriteRune(c)
	} else {
		r.UnreadRune()
	}
	for {
		c, _, _ := r.ReadRune()
		if !isDigit(c) {
			break
		}
		str.WriteRune(c)
	}
	_, err := strconv.Atoi(str.String())
	return str.String(), err
}

func skipBlank(r io.RuneScanner) {
	defer r.UnreadRune()
	for {
		c, _, _ := r.ReadRune()
		if !isBlank(c) {
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

func isArray(r rune) bool {
	return r == '['
}

func isGroup(r rune) bool {
	return r == '('
}
