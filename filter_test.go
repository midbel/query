package query

import (
	"os"
	"testing"
)

type QueryCase struct {
	Query string
	Want  string
}

func TestFilter(t *testing.T) {
	queries := []QueryCase{
		{
			Query: `.user`,
			Want:  `"midbel"`,
		},
		{
			Query: `.user,.mail`,
			Want:  `["midbel", "noreply@midbel.org"]`,
		},
		{
			Query: `{.user,.mail}`,
			Want:  `{"user": "midbel", "mail": "noreply@midbel.org"}`,
		},
		{
			Query: `{name: .user},{.user,.age}`,
			Want:  `[{"name": "midbel"}, {"user": "midbel", "age": 0}]`,
		}, {
			Query: `{.user,.age},{name: .user}`,
			Want:  `[{"user": "midbel", "age": 0}, {"name": "midbel"}]`,
		},
		{
			Query: `{name: .user, contact: .mail}`,
			Want:  `{"name": "midbel", "contact": "noreply@midbel.org"}`,
		},
		{
			Query: `[.user,.mail]`,
			Want:  `["midbel", "noreply@midbel.org"]`,
		},
		{
			Query: `{name: .user, projects: [.projects[].name]}`,
			Want:  `{"name": "midbel", "projects": ["slices", "charts", "query"]}`,
		}, {
			Query: `{name: .user, projects: [.projects[0, 1].name]}`,
			Want:  `[{"name": "midbel", "project": "slices"}, {"name": "midbel", "project": "charts"}]`,
		},
		{
			Query: `.projects[].priority`,
			Want:  `[10, 100, 60]`,
		},
		{
			Query: `.projects[0, 1].priority`,
			Want:  `[10, 100]`,
		},
		{
			Query: `.projects[0].priority`,
			Want:  `10`,
		},
	}
	for _, q := range queries {
		testQuery(t, q)
	}
}

func testQuery(t *testing.T, q QueryCase) {
	t.Helper()
	r, err := os.Open("testdata/sample.json")
	if err != nil {
		t.Fatalf("fail to open sample file")
	}
	defer r.Close()

	got, err := Filter(r, q.Query)
	if err != nil {
		t.Errorf("%s: unexpected error: %s", q.Query, err)
		return
	}
	if got != q.Want {
		t.Errorf("%s: result mismatched! want %s, got %s", q.Query, q.Want, got)
	}
}
