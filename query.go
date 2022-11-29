package query

import (
	"errors"
)

var ErrSkip = errors.New("skip")

var KeepAll Query = all{}

type all struct{}

func (a all) Next(string) (Query, error) {
	return a, nil
}

type array struct {
	index []string
	next  Query
}

func (a *array) Next(ident string) (Query, error) {
	if len(a.index) == 0 {
		return a.next, nil
	}
	for _, i := range a.index {
		if ident == i {
			return a.next, nil
		}
	}
	return nil, ErrSkip
}

type any struct {
	list []Query
}

func (a *any) Next(ident string) (Query, error) {
	for _, f := range a.list {
		if n, err := f.Next(ident); err == nil {
			return next(n), nil
		}
	}
	return nil, ErrSkip
}

type group struct {
	list []Query
	next Query
}

func (g *group) Next(ident string) (Query, error) {
	for _, f := range g.list {
		n, err := f.Next(ident)
		if err == nil {
			if n != nil {
				c := chain{
					curr: n,
					next: g.next,
				}
				return &c, nil
			}
			return n, nil
		}
	}
	return nil, ErrSkip
}

type ident struct {
	ident string
	next  Query
}

func (i *ident) Next(ident string) (Query, error) {
	if i.ident == ident {
		return next(i.next), nil
	}
	return nil, ErrSkip
}

type chain struct {
	curr Query
	next Query
}

func (c *chain) Next(ident string) (Query, error) {
	n, err := c.curr.Next(ident)
	if err == nil {
		if n == nil {
			return c.next, nil
		}
		c.curr = n
		return c, nil
	}
	return nil, err
}

func next(in Query) Query {
	// if in == nil {
	// 	return KeepAll
	// }
	return in
}
