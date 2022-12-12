package comma

import (
	"errors"
	"strings"
)

var ErrIndex = errors.New("index out of range")

type object struct {
	fields map[string]Indexer
	keys []string
}

func (o *object) Index(row []string) (string, error) {
	var str strings.Builder
	str.WriteRune('{')
	for i, k := range o.keys {
		if i > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}

		str.WriteRune('"')
		str.WriteString(k)
		str.WriteRune('"')
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

type literal struct {
	value string
}

func (i *literal) Index([]string) (string, error) {
	return i.value, nil
}

type index struct {
	index int
}

func (i *index) Index(row []string) (string, error) {
	if i.index < 0 || i.index >= len(row) {
		return "", ErrIndex
	}
	return row[i.index], nil
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
		str.WriteString(row[j])
	}
	str.WriteRune(']')
	return str.String(), nil
	return "", nil
}
