package comma

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrIndex = errors.New("index out of range")

type group struct {
	list []Indexer
}

func (g *group) Index(row []string) (string, error) {
	var str strings.Builder
	for i := range g.list {
		if i > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}

		got, err := g.list[i].Index(row)
		if err != nil {
			return "", err
		}
		str.WriteString(got)
	}
	return str.String(), nil
}

type object struct {
	fields map[string]Indexer
	keys   []string
}

func (o *object) Index(row []string) (string, error) {
	var str strings.Builder
	str.WriteRune('{')
	for i, k := range o.keys {
		if i > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}

		str.WriteString(withQuote(k, true))
		str.WriteRune(':')
		str.WriteRune(' ')

		val, err := o.fields[k].Index(row)
		if err != nil {
			return "", err
		}
		str.WriteString(val)
	}
	str.WriteRune('}')
	return str.String(), nil
}

type array struct {
	list []Indexer
}

func (a *array) Index(row []string) (string, error) {
	var str strings.Builder
	str.WriteRune('[')
	for i := range a.list {
		if i > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}
		got, err := a.list[i].Index(row)
		if err != nil {
			return "", err
		}
		str.WriteString(got)
	}
	str.WriteRune(']')
	return str.String(), nil
}

type set struct {
	index []Indexer
}

func (i *set) Index(row []string) (string, error) {
	var str strings.Builder
	str.WriteRune('[')
	for j := range i.index {
		if j > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}
		got, err := i.index[j].Index(row)
		if err != nil {
			return "", err
		}
		str.WriteString(got)
	}
	str.WriteRune(']')
	return str.String(), nil
}

type index struct {
	index int
}

func (i *index) Index(row []string) (string, error) {
	if i.index < 0 || i.index >= len(row) {
		return "", ErrIndex
	}
	return withQuote(row[i.index], false), nil
}

type interval struct {
	beg int
	end int
}

func (i *interval) Index(row []string) (string, error) {
	if i.end < i.beg {
		i.beg, i.end = i.end, i.beg
	}
	if i.beg < 0 || i.beg > len(row) {
		return "", ErrIndex
	}
	if i.end < 0 || i.end > len(row) {
		return "", ErrIndex
	}
	var (
		str strings.Builder
		pos int
	)
	str.WriteRune('[')
	for j := i.beg; j <= i.end; j++ {
		if pos > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}
		pos++
		str.WriteString(withQuote(row[j], false))
	}
	str.WriteRune(']')
	return str.String(), nil
	return "", nil
}

type literal struct {
	value string
}

func (i *literal) Index([]string) (string, error) {
	return withQuote(i.value, false), nil
}

func withQuote(str string, all bool) string {
	if str == "true" || str == "false" || str == "null" {
		return str
	}
	if str[0] == '"' && str[len(str)-1] == '"' {
		return str
	}
	if !all {
		_, err := strconv.ParseFloat(str, 64)
		if err == nil {
			return str
		}
	}
	return fmt.Sprintf("%q", str)
}
