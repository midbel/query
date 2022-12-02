package query

import (
	"strings"
	"testing"
)

func TestFilter(t *testing.T) {
	queries := []struct {
		Input string
		Query string
		Want  string
	}{
		{
			Input: `"foobar"`,
			Query: `.`,
			Want:  `"foobar"`,
		},
		{
			Input: `3.14e-15`,
			Query: `.`,
			Want:  `3.14e-15`,
		},
		{
			Input: `null`,
			Query: `.`,
			Want:  `null`,
		},
		{
			Input: `{"user": "foo bar"}`,
			Query: `.`,
			Want:  `{"user": "foo bar"}`,
		},
		{
			Input: `{"user": "foobar", "number": 42}`,
			Query: `.user`,
			Want:  `"foobar"`,
		},
		{
			Input: `{"user": {"name": "foobar", "age": 42, "active": true}}`,
			Query: `.user`,
			Want:  `{"name": "foobar", "age": 42, "active": true}`,
		},
		{
			Input: `[{"user": "foo"}, {"user": "bar"}]`,
			Query: `.[]`,
			Want:  `[{"user": "foo"}, {"user": "bar"}]`,
		},
		{
			Input: `[{"user": "foo"}, {"user": "bar"}]`,
			Query: `.[0,1]`,
			Want:  `[{"user": "foo"}, {"user": "bar"}]`,
		},
		{
			Input: `[{"user": "foo"}, {"user": "bar"}]`,
			Query: `.[0]`,
			Want:  `{"user": "foo"}`,
		},
		{
			Input: `{"user": "foobar", "number": 42, "active": false}`,
			Query: `.user,.active`,
			Want:  `["foobar", false]`,
		},
		{
			Input: `[{"user": "foo"}, {"user": "bar"}]`,
			Query: `.[].user`,
			Want:  `["foo", "bar"]`,
		},
		{
			Input: `[{"user": "foo"}, {"user": "bar"}]`,
			Query: `.[0].user`,
			Want:  `"foo"`,
		},
		{
			Input: `{"user": "foobar", "score": 42}`,
			Query: `{name: .user, .score}`,
			Want:  `{"name": "foobar", "score": 42}`,
		},
		{
			Input: `{"user": "foobar", "scores":[0.5,10.1,9]}`,
			Query: `{.user, score: .scores[]}`,
			Want:  `[{"user": "foobar", "score": 0.5}, {"user": "foobar", "score": 10.1}, {"user": "foobar", "score": 9}]`,
		},
		{
			Input: `{"user": "foobar", "scores":[0.5,10.1,9]}`,
			Query: `[.user, .scores[]]`,
			Want:  `["foobar", 0.5, 10.1, 9]`,
		},
		{
			Input: `{"user": "foobar", "scores": [{"name": "programming", "result": 0}, {"name": "testing", "result": 10}]}`,
			Query: `{.user, courses: [.scores[].name]}`,
			Want:  `{"user": "foobar", "courses": ["programming", "testing"]}`,
		},
	}
	for _, q := range queries {
		got, err := Filter(strings.NewReader(q.Input), q.Query)
		if err != nil {
			t.Errorf("%s: unexpected error: %s", q.Query, err)
			continue
		}
		if got != q.Want {
			t.Errorf("%q: result mismatched! want %s, got %s", q.Query, q.Want, got)
		}
	}
}
