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
	if f, err := os.Open(flag.Arg(1)); err == nil {
		defer f.Close()
		r = f
	} else {
		if flag.Arg(1) != "" {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}

	str, err := query.Execute(r, flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	fmt.Println(str)
}
