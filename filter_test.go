package query

import (
	"strings"
	"testing"
)

type QueryCase struct {
	Input string
	Query string
	Want  string
}

func TestExecute(t *testing.T) {
	t.Run("identity", testIdentity)
	t.Run("identifier", testIdentifier)
	t.Run("index", testIndex)
	t.Run("any", testAny)
	t.Run("object", testObject)
	t.Run("array", testArray)
	t.Run("pipeline", testPipeline)
}

func testIdentity(t *testing.T) {
	queries := []QueryCase{
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
			Want:  `{"user":"foo bar"}`,
		},
	}
	testQueries(t, queries)
}

func testIdentifier(t *testing.T) {
	queries := []QueryCase{
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
	}
	testQueries(t, queries)
}

func testIndex(t *testing.T) {
	queries := []QueryCase{
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
			Input: `[{"user": "foo"}, {"user": "bar"}]`,
			Query: `.[].user`,
			Want:  `["foo", "bar"]`,
		},
		{
			Input: `[{"user": "foo"}, {"user": "bar"}]`,
			Query: `.[0].user`,
			Want:  `"foo"`,
		},
	}
	testQueries(t, queries)
}

func testArray(t *testing.T) {
	queries := []QueryCase{
		{
			Input: `{"user": "foobar", "scores":[0.5,10.1,9]}`,
			Query: `[.user, .scores[]]`,
			Want:  `["foobar", 0.5, 10.1, 9]`,
		},
		{
			Input: `["foo", "bar"]`,
			Query: `[42, .[]]`,
			Want:  `[42, "foo", "bar"]`,
		},
	}
	testQueries(t, queries)
}

func testObject(t *testing.T) {
	queries := []QueryCase{
		{
			Input: `{"user": "foobar", "number": 42}`,
			Query: `{.user,age:42}`,
			Want:  `{"user": "foobar", "age": 42}`,
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
			Input: `{"user": "foobar", "scores": [{"name": "programming", "result": 0}, {"name": "testing", "result": 10}]}`,
			Query: `{.user, courses: [.scores[].name]}`,
			Want:  `{"user": "foobar", "courses": ["programming", "testing"]}`,
		},
	}
	testQueries(t, queries)
}

func testAny(t *testing.T) {
	queries := []QueryCase{
		{
			Input: `{"user": "foobar", "number": 42, "active": false}`,
			Query: `.user,.active`,
			Want:  `["foobar", false]`,
		},
	}
	testQueries(t, queries)
}

func testPipeline(t *testing.T) {
	queries := []QueryCase{
		{
			Input: `{"user": "foo bar"}`,
			Query: `. | .user`,
			Want:  `"foo bar"`,
		},
		{
			Input: `[{"user": "foo"}, {"user": "bar"}]`,
			Query: `.[] | {.user, age:42}`,
			Want:  `[{"user": "foo", "age": 42}, {"user": "bar", "age": 42}]`,
		},
		{
			Input: `{"user": {"name": "foo bar", "score": 42}}`,
			Query: `.user | {.score} | .score`,
			Want:  `42`,
		},
		{
			Input: `{"items": [{"name": "foo", "score": 10, "items": [{"name": "foo0", "score": 0}]}, {"name": "bar", "score": 5, "items": [{"name": "bar0", "score": 1}, {"name": "bar1", "score": 1}]}]}`,
			Query: `.items[] | {x: .name, y: .score, sub: [.items[] | {x: .name, y: .score}]}`,
			Want:  `[{"x": "foo", "y": 10, "sub": [{"x": "foo0", "y": 0}]}, {"x": "bar", "y": 5, "sub": [{"x": "bar0", "y": 1}, {"x": "bar1", "y": 1}]}]`,
		},
		{
			Input: `{"user": {"name": "foo bar", "score": 42}}`,
			Query: `.user | . | .score`,
			Want:  `42`,
		},
	}
	testQueries(t, queries)
}

func testQueries(t *testing.T, queries []QueryCase) {
	t.Helper()
	for _, q := range queries {
		got, err := Execute(strings.NewReader(q.Input), q.Query)
		if err != nil {
			t.Errorf("%s: unexpected error: %s", q.Query, err)
			continue
		}
		if got != q.Want {
			t.Errorf("%q: result mismatched! want %s, got %s", q.Query, q.Want, got)
		}
	}
}
