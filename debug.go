package query

import (
	"fmt"
	"io"
	"strings"
)

func Debug(w io.Writer, str string) error {
	q, err := Parse(str)
	if err != nil {
		return err
	}
	debug(w, q, 0)
	return nil
}

func debug(w io.Writer, q Query, level int) {
	if q == nil {
		return
	}
	prefix := strings.Repeat(" ", level*2)
	switch q := q.(type) {
	case *all:
		fmt.Fprintf(w, "%sall", prefix)
		fmt.Fprintln(w)
	case *ident:
		fmt.Fprintf(w, "%sident(%s)", prefix, q.ident)
		if q.next != nil {
			fmt.Fprintln(w, "[")
			debug(w, q.next, level+1)
			fmt.Fprintf(w, "%s]", prefix)
		}
		fmt.Fprintln(w)
	case *index:
		fmt.Fprintf(w, "%sindex(", prefix)
		for i := range q.list {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			fmt.Fprint(w, q.list[i])
		}
		fmt.Fprint(w, ")")
		if q.next != nil {
			fmt.Fprintln(w, "[")
			debug(w, q.next, level+1)
			fmt.Fprintf(w, "%s]", prefix)
		}
		fmt.Fprintln(w)
	case *any:
		fmt.Fprintf(w, "%sany[", prefix)
		fmt.Fprintln(w)
		for i := range q.list {
			debug(w, q.list[i], level+1)
		}
		fmt.Fprintf(w, "%s]", prefix)
		fmt.Fprintln(w)
	case *transform:
		indent := strings.Repeat(prefix, 2) + " "
		fmt.Fprintf(w, "%stransform()[", prefix)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%squery:", indent)
		fmt.Fprintln(w)
		debug(w, q.Query, level+1)
		fmt.Fprintf(w, "%snext:", indent)
		fmt.Fprintln(w)
		debug(w, q.next, level+1)
		fmt.Fprintf(w, "%s]", prefix)
		fmt.Fprintln(w)
	case *object:
		indent := strings.Repeat(prefix, 2) + " "
		fmt.Fprintf(w, "%sobject()[", prefix)
		fmt.Fprintln(w)
		for k, v := range q.fields {
			fmt.Fprintf(w, "%skey(%s):", indent, k)
			fmt.Fprintln(w)
			debug(w, v, level+1)
		}
		fmt.Fprintf(w, "%s]", prefix)
		fmt.Fprintln(w)
	case *array:
		fmt.Fprintf(w, "%sarray()[", prefix)
		fmt.Fprintln(w)
		for i := range q.list {
			debug(w, q.list[i], level+1)
		}
		fmt.Fprintf(w, "%s]", prefix)
		fmt.Fprintln(w)
	default:
		fmt.Fprintf(w, "%T", q)
		fmt.Fprintln(w)
	}
}
