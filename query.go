package query

import (
	"errors"
)

var ErrSkip = errors.New("skip")

var KeepAll Filter = all{}

type all struct{}

func (a all) Next(string) (Filter, error) {
	return a, nil
}

type array struct {
	index []int
	next  Filter
}

func (a *array) Next(ident string) (Filter, error) {
	return nil, ErrSkip
}

type any struct {
	list []Filter
}

func (a *any) Next(ident string) (Filter, error) {
	for _, f := range a.list {
		if n, err := f.Next(ident); err == nil {
			return next(n), nil
		}
	}
	return nil, ErrSkip
}

type group struct {
	list []Filter
	next Filter
}

func (g *group) Next(ident string) (Filter, error) {
	return nil, ErrSkip
}

type ident struct {
	ident string
	next  Filter
}

func (i *ident) Next(ident string) (Filter, error) {
	if i.ident == ident {
		return next(i.next), nil
	}
	return nil, ErrSkip
}

func next(in Filter) Filter {
	if in == nil {
		return KeepAll
	}
	return in
}
