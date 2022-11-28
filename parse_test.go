package query

import (
	"testing"
)

func TestParse(t *testing.T) {
	data := []string{
		".",
		".ident",
		".\"ident\"",
		".'ident'",
		".first.last",
		".first,.last",
		".[]",
		".[0, 1, 2]",
		".array[]",
		"(.first,.last).ident",
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
		"ident",
		".ident.",
		"._ident",
		".1ident",
		".first,.last,",
	}
	for _, d := range data {
		_, err := Parse(d)
		if err == nil {
			t.Errorf("%s: invalid query parsed successfully", d)
		}
	}
}
