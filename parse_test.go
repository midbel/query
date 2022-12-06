package query

import (
	"testing"
)

func TestParse(t *testing.T) {
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
