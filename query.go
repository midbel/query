package query

import (
	"errors"
	"strings"

	"github.com/midbel/slices"
)

type Query interface {
	Next(string) (Query, error)
	Get() string
	get() []string
}

type setter interface {
	set(string)
}

var ErrSkip = errors.New("skip")

var KeepAll Query = &all{}

type all struct {
	value string
}

func (a *all) Next(string) (Query, error) {
	return nil, nil
}

func (a *all) Get() string {
	return a.value
}

func (a *all) get() []string {
	return []string{a.value}
}

func (a *all) set(str string) {
	a.value = str
}

type index struct {
	list   []string
	values []string
	next   Query
}

func (i *index) Next(ident string) (Query, error) {
	if len(i.list) == 0 {
		return i.next, nil
	}
	for _, j := range i.list {
		if ident == j {
			return i.next, nil
		}
	}
	return nil, ErrSkip
}

func (i *index) Get() string {
	var str strings.Builder
	str.WriteRune('[')
	for j := range i.values {
		if j > 0 {
			str.WriteRune(',')
		}
		str.WriteString(i.values[j])
	}
	str.WriteRune(']')
	return str.String()
}

func (i *index) get() []string {
	return i.values
}

func (i *index) set(str string) {
	i.values = append(i.values, str)
}

type any struct {
	list   []Query
	values []string
}

func (a *any) Next(ident string) (Query, error) {
	for _, f := range a.list {
		if n, err := f.Next(ident); err == nil {
			return n, nil
		}
	}
	return nil, ErrSkip
}

func (a *any) Get() string {
	var str strings.Builder
	str.WriteRune('[')
	for i := range a.values {
		if i > 0 {
			str.WriteRune(',')
		}
		str.WriteString(a.values[i])
	}
	str.WriteRune(']')
	return str.String()
}

func (a *any) set(str string) {
	a.values = append(a.values, str)
}

func (a *any) get() []string {
	return a.values
}

type ident struct {
	ident string
	value string
	next  Query
}

func (i *ident) Next(ident string) (Query, error) {
	if i.ident == ident {
		return i.next, nil
	}
	return nil, ErrSkip
}

func (i *ident) Get() string {
	if i.next != nil {
		return i.next.Get()
	}
	return i.value
}

func (i *ident) set(str string) {
	i.value = str
}

func (i *ident) get() []string {
	list := []string{i.value}
	if i.next == nil {
		return list
	}
	return i.next.get()
}

type array struct {
	list []Query
	last Query
}

func (a *array) Next(ident string) (Query, error) {
	for _, q := range a.list {
		n, err := q.Next(ident)
		if err == nil {
			a.last = q
			return n, nil
		}
	}
	return nil, ErrSkip
}

func (a *array) Get() string {
	var str strings.Builder
	str.WriteRune('[')
	for i := range a.list {
		if i > 0 {
			str.WriteRune(',')
		}
		vs := a.list[i].get()
		for i := range vs {
			if i > 0 {
				str.WriteRune(',')
			}
			str.WriteString(vs[i])
		}
	}
	str.WriteRune(']')
	return str.String()
}

func (a *array) get() []string {
	var values []string
	for i := range a.list {
		values = append(values, a.list[i].get()...)
	}
	return values
}

func (a *array) set(str string) {
	if s, ok := a.last.(setter); ok {
		s.set(str)
		a.last = nil
	}
}

type object struct {
	fields map[string]Query
	keys   []string
}

func (o *object) Next(ident string) (Query, error) {
	for k, q := range o.fields {
		n, err := q.Next(ident)
		if err == nil {
			o.keys = append(o.keys, k)
			return n, err
		}
	}
	return nil, ErrSkip
}

func (o *object) Get() string {
	var (
		values [][]string
		str    strings.Builder
	)
	for _, k := range o.keys {
		q := o.fields[k]
		values = append(values, q.get())
	}
	values = slices.Combine(values...)
	if len(values) > 1 {
		str.WriteRune('[')
	}
	for i, vs := range values {
		if i > 0 {
			str.WriteRune(',')
		}
		str.WriteRune('{')
		for j, k := range o.keys {
			if j > 0 {
				str.WriteRune(',')
			}
			str.WriteRune('"')
			str.WriteString(k)
			str.WriteRune('"')
			str.WriteRune(':')
			str.WriteString(vs[j])
		}
		str.WriteRune('}')
	}
	if len(values) > 1 {
		str.WriteRune(']')
	}
	return str.String()
}

func (o *object) set(str string) {
	k := slices.Lst(o.keys)
	if s, ok := o.fields[k].(setter); ok {
		s.set(str)
	}
}

func (o *object) get() []string {
	var values []string
	for _, k := range o.keys {
		q := o.fields[k]
		values = append(values, q.get()...)
	}
	return values
}
