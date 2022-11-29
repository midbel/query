package query

type all struct{}

func (a all) Next(string) (Filter, error) {
	return a, nil
}

type array struct {
	index []string
	next  Filter
}

func (a *array) Next(ident string) (Filter, error) {
	if len(a.index) == 0 {
		return next(a.next), nil
	}
	for i := range a.index {
		if a.index[i] == ident {
			return next(a.next), nil
		}
	}
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

type chain []Filter

func (c *chain) Next(ident string) (Filter, error) {
	if len(*c) == 0 {
		return KeepAll, nil
	}
	n, err := (*c)[0].Next(ident)
	if err != nil {
		return nil, err
	}
	if n != KeepAll {
		(*c)[0] = n
		return c, nil
	}
	*c = (*c)[1:]
	return c, nil
}

type group struct {
	list []Filter
	next Filter
}

func (g *group) Next(ident string) (Filter, error) {
	for _, f := range g.list {
		if n, err := f.Next(ident); err == nil {
			cs := chain{
				next(n),
				g.next,
			}
			return &cs, nil
		}
	}
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
