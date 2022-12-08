package query

import (
	"fmt"
	"testing"
)

func TestParse(t *testing.T) {
	data := []struct {
		Input string
		Want  Query
	}{
		{
			Input: `.`,
			Want:  All(),
		},
		{
			Input: `. | .foobar`,
			Want:  nil,
		},
		{
			Input: `.foo | . | .bar`,
			Want:  nil,
		},
		{
			Input: `.foo | .bar | .`,
			Want:  nil,
		},
		{
			Input: `.foo | .foo, .bar | .bar`,
			Want:  Any(PipeLine(Ident("foo"), Ident("foo")), PipeLine(Ident("bar"), Ident("bar"))),
		},
		{
			Input: `.foobar`,
			Want:  Ident("foobar"),
		},
		{
			Input: `.foo.bar`,
			Want:  IdentNext("foo", Ident("bar")),
		},
		{
			Input: `.[]`,
			Want:  Index(nil),
		},
		{
			Input: `.[1, 2, 3]`,
			Want:  Index([]string{"1", "2", "3"}),
		},
		{
			Input: `.[].foobar`,
			Want:  IndexNext(nil, Ident("foobar")),
		},
		{
			Input: `.[1, 2].foobar`,
			Want:  IndexNext([]string{"1", "2"}, Ident("foobar")),
		},
		{
			Input: `.list[]`,
			Want:  IdentNext("list", Index(nil)),
		},
		{
			Input: `.list[].foobar`,
			Want:  IdentNext("list", IndexNext(nil, Ident("foobar"))),
		},
		{
			Input: `.foo | .bar`,
			Want:  PipeLine(Ident("foo"), Ident("bar")),
		},
		{
			Input: `.foo.bar | .bar`,
			Want:  IdentNext("foo", PipeLine(Ident("bar"), Ident("bar"))),
		},
		{
			Input: `.[] | .foo | .bar`,
			Want:  PipeLine(Index(nil), Ident("foo"), Ident("bar")),
		},
		{
			Input: `.foo,.bar`,
			Want:  Any(Ident("foo"), Ident("bar")),
		},
		{
			Input: `.foo[],.bar`,
			Want:  Any(IdentNext("foo", Index(nil)), Ident("bar")),
		},
		{
			Input: `.foo | .bar, .bar | .foo`,
			Want:  Any(PipeLine(Ident("foo"), Ident("bar")), PipeLine(Ident("bar"), Ident("foo"))),
		},
		{
			Input: `{foo: .foo, bar: .bar}`,
			Want:  Object([]string{"foo", "bar"}, []Query{Ident("foo"), Ident("bar")}),
		},
		{
			Input: `{.foo,.bar}`,
			Want:  Object([]string{"foo", "bar"}, []Query{Ident("foo"), Ident("bar")}),
		},
		{
			Input: `.foo | {.foo,.bar} | .bar`,
			Want:  PipeLine(Ident("foo"), Object([]string{"foo", "bar"}, []Query{Ident("foo"), Ident("bar")}), Ident("bar")),
		},
		{
			Input: `[.foo, .bar]`,
			Want:  Array(Ident("foo"), Ident("bar")),
		},
		{
			Input: `[[.foo],.bar]`,
			Want:  Array(Array(Ident("foo")), Ident("bar")),
		},
		{
			Input: `.foo.bar | [.foo0, .bar0] | .bar1`,
			Want:  IdentNext("foo", PipeLine(Ident("bar"), Array(Ident("foo0"), Ident("bar0")), Ident("bar1"))),
		},
		{
			Input: `[.foo, .bar] | .foobar`,
			Want:  PipeLine(Array(Ident("foo"), Ident("bar")), Ident("foobar")),
		},
		{
			Input: `.foobar | [.foo, .bar]`,
			Want:  PipeLine(Ident("foobar"), Array(Ident("foo"), Ident("bar"))),
		},
	}
	for _, d := range data {
		got, err := Parse(d.Input)
		if err != nil {
			t.Errorf("%s: error parsing query! %s", d.Input, err)
			continue
		}
		if err := cmpQuery(d.Want, got); err != nil {
			t.Errorf("%s: queries mismatched! %s", d.Input, err)
		}
	}
}

func cmpQuery(q, other Query) error {
	switch q.(type) {
	case *ident:
		return cmpIdent(q, other)
	case *index:
		return cmpIndex(q, other)
	case *all:
		return cmpAll(q, other)
	case *pipeline:
		return cmpPipe(q, other)
	case *any:
		return cmpAny(q, other)
	case *array:
		return cmpArray(q, other)
	case *object:
		return cmpObject(q, other)
	default:
		return fmt.Errorf("unsupported query type %T", q)
	}
}

func cmpArray(q, other Query) error {
	i, ok := q.(*array)
	if !ok {
		return fmt.Errorf("array: unexpected query type %T", q)
	}
	j, ok := other.(*array)
	if !ok {
		return fmt.Errorf("array: unexpected query type %T", other)
	}
	if len(i.list) != len(j.list) {
		return fmt.Errorf("array: length mismatched! %d >< %d", len(i.list), len(j.list))
	}
	for k := range i.list {
		if err := cmpQuery(i.list[k], j.list[k]); err != nil {
			return err
		}
	}
	return nil
}

func cmpObject(q, other Query) error {
	i, ok := q.(*object)
	if !ok {
		return fmt.Errorf("object: unexpected query type %T", q)
	}
	j, ok := other.(*object)
	if !ok {
		return fmt.Errorf("object: unexpected query type %T", other)
	}
	if len(i.fields) != len(j.fields) {
		return fmt.Errorf("object: length mismatched! %d >< %d", len(i.fields), len(j.fields))
	}
	for k := range i.fields {
		if err := cmpQuery(i.fields[k], j.fields[k]); err != nil {
			return err
		}
	}
	return nil
}

func cmpAny(q, other Query) error {
	i, ok := q.(*any)
	if !ok {
		return fmt.Errorf("any: unexpected query type %T", q)
	}
	j, ok := other.(*any)
	if !ok {
		return fmt.Errorf("any: unexpected query type %T", other)
	}
	if len(i.list) != len(j.list) {
		return fmt.Errorf("any: length mismatched! %d >< %d", len(i.list), len(j.list))
	}
	for k := range i.list {
		if err := cmpQuery(i.list[k], j.list[k]); err != nil {
			return err
		}
	}
	return nil
}

func cmpIdent(q, other Query) error {
	i, ok := q.(*ident)
	if !ok {
		return fmt.Errorf("ident: unexpected query type %T", q)
	}
	j, ok := other.(*ident)
	if !ok {
		return fmt.Errorf("ident: unexpected query type %T", other)
	}
	if i.ident != j.ident {
		return fmt.Errorf("ident: identifier mismatched! %s >< %s", i.ident, j.ident)
	}
	if i.next == nil && j.next == nil {
		return nil
	}
	return cmpQuery(i.next, j.next)
}

func cmpIndex(q, other Query) error {
	i, ok := q.(*index)
	if !ok {
		return fmt.Errorf("index: unexpected query type %T", q)
	}
	j, ok := other.(*index)
	if !ok {
		return fmt.Errorf("index: unexpected query type %T", other)
	}
	if len(i.list) != len(j.list) {
		return fmt.Errorf("index: length mismatched! %d >< %d", len(i.list), len(j.list))
	}
	for k := range i.list {
		if i.list[k] != j.list[k] {
			return fmt.Errorf("index: element mismatched! %s >< %s", i.list[k], j.list[k])
		}
	}
	if i.next == nil && j.next == nil {
		return nil
	}
	return cmpQuery(i.next, j.next)
}

func cmpPipe(q, other Query) error {
	i, ok := q.(*pipeline)
	if !ok {
		return fmt.Errorf("pipe: unexpected query type %T", q)
	}
	j, ok := other.(*pipeline)
	if !ok {
		return fmt.Errorf("pipe: unexpected query type %T", other)
	}
	if err := cmpQuery(i.Query, j.Query); err != nil {
		return err
	}
	if len(i.queries) != len(j.queries) {
		return fmt.Errorf("pipe: length mismatched! %d >< %d", len(i.queries), len(j.queries))
	}
	for k := range i.queries {
		if err := cmpQuery(i.queries[k], j.queries[k]); err != nil {
			return err
		}
	}
	return nil
}

func cmpAll(q, other Query) error {
	if _, ok := q.(*all); !ok {
		return fmt.Errorf("all: unexpected query type %T", q)
	}
	if _, ok := other.(*all); !ok {
		return fmt.Errorf("all: unexpected query type %T", other)
	}
	return nil
}

func TestParseBase(t *testing.T) {
	data := []string{
		`.`,
		`. | .ident`,
		`.ident | .ident`,
		`.ident`,
		`."ident"`,
		`.'ident'`,
		`.'ident'[]`,
		`.'parent'."child"`,
		`.first.last`,
		`.first,.last`,
		`.[]`,
		`.[0, 1, 2]`,
		`.array[]`,
		`.array[].ident`,
		`.`,
		`{}`,
		`{ident: .ident}`,
		`{.ident}`,
		`[]`,
		`[.ident]`,
		`[.ident] | {data: .ident} | .data`,
		`.ident[] | {x: .ident, y: (.ident | .ident)}`,
		`[.ident, (.ident | .ident), .ident]`,
	}
	for _, d := range data {
		_, err := Parse(d)
		if err != nil {
			t.Errorf("%s: parse error: %s", d, err)
		}
	}
}

func TestParse_Error(t *testing.T) {
	data := []string{
		`. |`,
		`|`,
		`ident`,
		`.ident.`,
		`._ident`,
		`.1ident`,
		`.first,.last,`,
		`.'first`,
		`.array[1, 2`,
		`.array[`,
		`.array[1 2`,
		`.[`,
		`.]`,
		`.array["foobar"]`,
	}
	for _, d := range data {
		_, err := Parse(d)
		if err == nil {
			t.Errorf("%s: invalid query parsed successfully", d)
		}
	}
}
