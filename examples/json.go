package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/query"
)

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer r.Close()

	rs, err := query.Filter(r, flag.Arg(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for i := range rs {
		fmt.Println(rs[i])
	}
}
