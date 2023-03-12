package query

import (
	"errors"
	"fmt"
	"strings"

	"github.com/midbel/slices"
)

const Identity = "."

var errSkip = errors.New("skip")

type Query interface {
	fmt.Stringer

	Next(string) (Query, error)
	Clone() Query
}

type pipeline struct {
	Query
	queries []Query
}

func PipeLine(q Query, next ...Query) Query {
	return &pipeline{
		Query:   q,
		queries: next,
	}
}

func (p *pipeline) Clone() Query {
	var q pipeline
	q.Query = p.Query.Clone()
	for i := range p.queries {
		q.queries = append(q.queries, p.queries[i].Clone())
	}
	return &q
}

type all struct{}

func All() Query {
	var q all
	return &q
}

func (a *all) Next(string) (Query, error) {
	return nil, nil
}

func (a *all) String() string {
	return Identity
}

func (a *all) Clone() Query {
	var q all
	return &q
}

type ptr struct {
	Query
}

func Pointer(q Query) Query {
	return &ptr{
		Query: q,
	}
}

func cloneQuery(q Query) Query {
	if p, ok := q.(*ptr); ok {
		return p.Query.Clone()
	}
	return q
}

func (p *ptr) Clone() Query {
	return p
}

type recurse struct {
	Query
}

func Recurse(q Query) Query {
	return &recurse{
		Query: q,
	}
}

func (r *recurse) Next(ident string) (Query, error) {
	n, err := r.Query.Next(ident)
	if err != nil {
		return r, nil
	}
	return n, err
}

func (r *recurse) Clone() Query {
	var q recurse
	q.Query = r.Query.Clone()
	return &q
}

type literal struct {
	value string
}

func Value(str string) Query {
	return &literal{
		value: str,
	}
}

func (i *literal) Next(string) (Query, error) {
	return nil, errSkip
}

func (i *literal) String() string {
	return i.value
}

func (i *literal) Clone() Query {
	q := *i
	return &q
}

type ident struct {
	ident string
	next  Query
}

func Ident(key string) Query {
	return IdentNext(key, nil)
}

func IdentNext(key string, next Query) Query {
	return &ident{
		ident: key,
		next:  next,
	}
}

func (i *ident) Next(ident string) (Query, error) {
	if i.ident == ident {
		return i.next, nil
	}
	return nil, errSkip
}

func (i *ident) String() string {
	return i.ident
}

func (i *ident) Clone() Query {
	var q ident
	q.ident = i.ident
	if i.next != nil {
		q.next = i.next.Clone()
	}
	return &q
}

type index struct {
	list []string
	next Query
}

func Index(list []string) Query {
	return IndexNext(list, nil)
}

func IndexNext(list []string, next Query) Query {
	return &index{
		list: list,
		next: next,
	}
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
	return nil, errSkip
}

func (i *index) String() string {
	return strings.Join(i.list, ",")
}

func (i *index) Clone() Query {
	var q index
	q.list = make([]string, len(i.list))
	copy(q.list, i.list)
	if i.next != nil {
		q.next = i.next.Clone()
	}
	return &q
}

type any struct {
	list []Query
	last Query
}

func Any(list ...Query) Query {
	return &any{
		list: list,
	}
}

func (a *any) Next(ident string) (Query, error) {
	for _, f := range a.list {
		if n, err := f.Next(ident); err == nil {
			a.last = f
			return n, nil
		}
	}
	return nil, errSkip
}

func (a *any) String() string {
	var values []string
	for i := range a.list {
		values = append(values, a.list[i].String())
	}
	return writeArray(values)
}

func (a *any) reset() {
	a.last = nil
}

func (a *any) Clone() Query {
	var q any
	for i := range a.list {
		q.list = append(q.list, a.list[i].Clone())
	}
	return &q
}

type array struct {
	list []Query
	last Query
}

func Array(list ...Query) Query {
	return &array{
		list: list,
	}
}

func (a *array) Next(ident string) (Query, error) {
	for i := range a.list {
		a.list[i] = cloneQuery(a.list[i])
		n, err := a.list[i].Next(ident)
		if err == nil {
			a.last = a.list[i]
			return n, nil
		}
	}
	return nil, errSkip
}

func (a *array) String() string {
	var values []string
	return writeArray(values)
}

func (a *array) Clone() Query {
	var q array
	for i := range a.list {
		q.list = append(q.list, a.list[i].Clone())
	}
	return &q
}

type object struct {
	fields map[string]Query
	keys   []string
}

func Object(ks []string, qs []Query) Query {
	var obj object
	obj.fields = make(map[string]Query)
	for i, k := range ks {
		if i >= len(qs) {
			break
		}
		obj.fields[k] = qs[i]
	}
	return &obj
}

func (o *object) Next(ident string) (Query, error) {
	for k := range o.fields {
		o.fields[k] = cloneQuery(o.fields[k])
		n, err := o.fields[k].Next(ident)
		if err == nil {
			o.keys = append(o.keys, k)
			return n, err
		}
	}
	return nil, errSkip
}

func (o *object) String() string {
	var (
		values [][]string
		keys   []string
	)
	return writeObject(keys, slices.Combine(values...))
}

func (o *object) Clone() Query {
	var q object
	q.fields = make(map[string]Query)
	for k := range o.fields {
		q.fields[k] = o.fields[k].Clone()
	}
	return &q
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
			if j < len(vs) {
				str.WriteString(vs[j])
			} else {
				str.WriteString("null")
			}
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

func keepAll(q Query) bool {
	_, ok := q.(*all)
	return ok
}
