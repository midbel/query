package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/midbel/query"
)

func main() {
	flag.Parse()

	var r io.Reader = os.Stdin
	if f, err := os.Open(flag.Arg(0)); err == nil {
		defer f.Close()
		r = f
	} else {
		if flag.Arg(0) != "" && flag.Arg(0) != "-" {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}

	res, err := query.Filter(r, flag.Arg(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf(">> %+s\n", res)
}
