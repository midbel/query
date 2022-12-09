package query

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func Debug(w io.Writer, str string) error {
	q, err := Parse(str)
	if err != nil {
		return err
	}
	ws := bufio.NewWriter(w)
	defer ws.Flush()
	debug(ws, q, 0, false)
	return nil
}

func debug(w io.Writer, q Query, level int, nonl bool) {
	if q == nil {
		return
	}
	var (
		prefix = strings.Repeat(" ", level*2)
		header string
	)
	if !nonl {
		header = prefix
	}
	switch q := q.(type) {
	case *all:
		fmt.Fprintf(w, "%sall", header)
		fmt.Fprintln(w)
	case *ident:
		fmt.Fprintf(w, "%sident(%s)", header, q.ident)
		if q.next != nil {
			fmt.Fprintln(w, " [")
			debug(w, q.next, level+1, false)
			fmt.Fprintf(w, "%s]", prefix)
		}
		fmt.Fprintln(w)
	case *index:
		fmt.Fprintf(w, "%sindex(", header)
		for i := range q.list {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			fmt.Fprint(w, q.list[i])
		}
		fmt.Fprint(w, ")")
		if q.next != nil {
			fmt.Fprintln(w, " [")
			debug(w, q.next, level+1, false)
			fmt.Fprintf(w, "%s]", prefix)
		}
		fmt.Fprintln(w)
	case *any:
		fmt.Fprintf(w, "%sany [", header)
		fmt.Fprintln(w)
		for i := range q.list {
			debug(w, q.list[i], level+1, false)
		}
		fmt.Fprintf(w, "%s]", prefix)
		fmt.Fprintln(w)
	case *pipeline:
		fmt.Fprintf(w, "%spipeline [", header)
		fmt.Fprintln(w)
		debug(w, q.Query, level+1, false)
		for i := range q.queries {
			debug(w, q.queries[i], level+1, false)
		}
		fmt.Fprintf(w, "%s]", prefix)
		fmt.Fprintln(w)
	case *object:
		fmt.Fprintf(w, "%sobject [", header)
		fmt.Fprintln(w)
		for k, v := range q.fields {
			fmt.Fprintf(w, "%skey(%s): ", prefix+" - ", k)
			debug(w, v, level+1, true)
		}
		fmt.Fprintf(w, "%s]", prefix)
		fmt.Fprintln(w)
	case *array:
		fmt.Fprintf(w, "%sarray [", header)
		fmt.Fprintln(w)
		for i := range q.list {
			debug(w, q.list[i], level+1, false)
		}
		fmt.Fprintf(w, "%s]", prefix)
		fmt.Fprintln(w)
	default:
		fmt.Fprintf(w, "%T", q)
		fmt.Fprintln(w)
	}
}
