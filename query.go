package query

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

type F interface {
	Filter(io.Reader) (io.Reader, error)
}

var ErrSkip = errors.New("skip")

var (
	keepAll Query = &all{}
	discard Query = &all{}
)

type chain struct {
	queries []Query
}

func (c *chain) At(n int) (string, error) {
	if n < 0 || n >= len(c.queries) {
		return "", fmt.Errorf("bad index")
	}
	return slices.At(c.queries, n).Get(), nil
}

func (c *chain) Filter(r io.Reader) (io.Reader, error) {
	for _, q := range c.queries {
		err := Execute(r, q)
		if err != nil {
			return nil, err
		}
		got := q.Get()
		r = bytes.NewReader([]byte(got))
	}
	return r, nil
}

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

type ident struct {
	ident  string
	values []string
	next   Query
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
	if len(i.values) == 1 {
		return slices.Fst(i.values)
	}
	return writeArray(i.values)
}

func (i *ident) set(str string) {
	i.values = append(i.values, str)
}

func (i *ident) get() []string {
	if i.next == nil {
		return i.values
	}
	return i.next.get()
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
	_ = fmt.Sprintf
	if i.next != nil {
		return i.next.Get()
	}
	if len(i.values) == 1 {
		return slices.Fst(i.values)
	}
	return writeArray(i.values)
}

func (i *index) get() []string {
	if i.next == nil {
		return i.values
	}
	return i.next.get()
}

func (i *index) set(str string) {
	i.values = append(i.values, str)
}

type any struct {
	list []Query
	last Query
}

func (a *any) Next(ident string) (Query, error) {
	for _, f := range a.list {
		if n, err := f.Next(ident); err == nil {
			a.last = f
			return n, nil
		}
	}
	return nil, ErrSkip
}

func (a *any) Get() string {
	var values []string
	for i := range a.list {
		values = append(values, a.list[i].Get())
	}
	return writeArray(values)
}

func (a *any) set(str string) {
	if s, ok := a.last.(setter); ok {
		s.set(str)
		a.last = nil
	}
}

func (a *any) get() []string {
	var values []string
	for i := range a.list {
		arr := writeArray(a.list[i].get())
		values = append(values, arr)
	}
	return values
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
	var values []string
	for i := range a.list {
		values = append(values, a.list[i].get()...)
	}
	return writeArray(values)
}

func (a *array) get() []string {
	var values []string
	for i := range a.list {
		arr := writeArray(a.list[i].get())
		values = append(values, arr)
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
	var values [][]string
	for _, k := range o.keys {
		q := o.fields[k]
		values = append(values, q.get())
	}
	return writeObject(o.keys, slices.Combine(values...))
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

func writeObject(keys []string, values [][]string) string {
	var str strings.Builder
	if len(values) > 1 {
		str.WriteRune('[')
	}
	for i, vs := range values {
		if i > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}
		str.WriteRune('{')
		for j, k := range keys {
			if j > 0 {
				str.WriteRune(',')
				str.WriteRune(' ')
			}
			str.WriteRune('"')
			str.WriteString(k)
			str.WriteRune('"')
			str.WriteRune(':')
			str.WriteRune(' ')
			str.WriteString(vs[j])
		}
		str.WriteRune('}')
	}
	if len(values) > 1 {
		str.WriteRune(']')
	}
	return str.String()
}

func writeArray(values []string) string {
	var str strings.Builder
	str.WriteRune('[')
	for i := range values {
		if i > 0 {
			str.WriteRune(',')
			str.WriteRune(' ')
		}
		str.WriteString(values[i])
	}
	str.WriteRune(']')
	return str.String()
}
