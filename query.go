package query

import (
	"errors"
	"fmt"
	"strings"

	"github.com/midbel/slices"
)

const Identity = "."

type Query interface {
	fmt.Stringer

	Next(string) (Query, error)
	Get() []string
	update(string) error
	clear()
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

func (p *pipeline) update(str string) error {
	for i := range p.queries {
		r := strings.NewReader(str)
		p.queries[i].clear()

		if err := execute(r, p.queries[i]); err != nil {
			return err
		}
		str = p.queries[i].String()
	}
	err := p.Query.update(str)
	return err
}

func (p *pipeline) Clone() Query {
	var q pipeline
	q.Query = p.Query.Clone()
	for i := range p.queries {
		q.queries = append(q.queries, p.queries[i].Clone())
	}
	return &q
}

var errSkip = errors.New("skip")

type all struct {
	value string
}

func All() Query {
	var q all
	return &q
}

func (a *all) Next(string) (Query, error) {
	return nil, nil
}

func (a *all) String() string {
	return a.value
}

func (a *all) Get() []string {
	return []string{a.value}
}

func (a *all) update(str string) error {
	a.value = str
	return nil
}

func (a *all) clear() {
	a.value = ""
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

func (p *ptr) clear() {
	// noop
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

func (i *literal) Get() []string {
	return []string{i.value}
}

func (i *literal) update(string) error {
	return fmt.Errorf("literal query can not be updated")
}

func (i *literal) clear() {
	// noop
}

func (i *literal) Clone() Query {
	q := *i
	return &q
}

type ident struct {
	ident  string
	values []string
	next   Query
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
	if i.next != nil {
		return i.next.String()
	}
	if len(i.values) == 1 {
		return slices.Fst(i.values)
	}
	return writeArray(i.values)
}

func (i *ident) Get() []string {
	if i.next == nil {
		return i.values
	}
	return i.next.Get()
}

func (i *ident) update(str string) error {
	i.values = append(i.values, str)
	return nil
}

func (i *ident) clear() {
	i.values = i.values[:0]
	if i.next != nil {
		i.next.clear()
	}
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
	list   []string
	values []string
	next   Query
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
	if i.next != nil {
		return i.next.String()
	}
	if len(i.values) == 1 {
		return slices.Fst(i.values)
	}
	return writeArray(i.values)
}

func (i *index) Get() []string {
	if i.next == nil {
		return i.values
	}
	return i.next.Get()
}

func (i *index) update(str string) error {
	i.values = append(i.values, str)
	return nil
}

func (i *index) clear() {
	i.values = i.values[:0]
	if i.next != nil {
		i.next.clear()
	}
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

func (a *any) Get() []string {
	var values []string
	for i := range a.list {
		arr := writeArray(a.list[i].Get())
		values = append(values, arr)
	}
	return values
}

func (a *any) update(str string) error {
	if a.last == nil {
		return fmt.Errorf("no query selected")
	}
	defer a.reset()
	return a.last.update(str)
}

func (a *any) clear() {
	for i := range a.list {
		a.list[i].clear()
	}
	a.reset()
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
	for i := range a.list {
		values = append(values, a.list[i].Get()...)
	}
	return writeArray(values)
}

func (a *array) Get() []string {
	var values []string
	for i := range a.list {
		arr := writeArray(a.list[i].Get())
		values = append(values, arr)
	}
	return values
}

func (a *array) update(str string) error {
	if a.last == nil {
		return fmt.Errorf("no query selected")
	}
	defer a.reset()
	return a.last.update(str)
}

func (a *array) clear() {
	for i := range a.list {
		a.list[i].clear()
	}
	a.reset()
}

func (a *array) Clone() Query {
	var q array
	for i := range a.list {
		q.list = append(q.list, a.list[i].Clone())
	}
	return &q
}

func (a *array) reset() {
	a.last = nil
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
	for _, k := range o.keys {
		q := o.fields[k]
		values = append(values, q.Get())
		keys = append(keys, k)
	}
	for k, q := range o.fields {
		if _, ok := q.(*literal); ok {
			keys = append(keys, k)
			values = append(values, q.Get())
		}
	}
	return writeObject(keys, slices.Combine(values...))
}

func (o *object) Get() []string {
	var (
		values [][]string
		keys   []string
	)
	for _, k := range o.keys {
		q := o.fields[k]
		values = append(values, q.Get())
		keys = append(keys, k)
	}
	for k, q := range o.fields {
		if _, ok := q.(*literal); ok {
			keys = append(keys, k)
			values = append(values, q.Get())
		}
	}
	var list []string
	for _, vs := range slices.Combine(values...) {
		str := writeObject(keys, [][]string{vs})
		list = append(list, str)
	}
	return list
}

func (o *object) update(str string) error {
	if len(o.keys) == 0 {
		return fmt.Errorf("no query selected")
	}
	k := slices.Lst(o.keys)
	q, ok := o.fields[k]
	if !ok {
		return fmt.Errorf("no query selected for key %s", k)
	}
	return q.update(str)
}

func (o *object) clear() {
	for _, q := range o.fields {
		q.clear()
	}
	o.keys = o.keys[:0]
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
