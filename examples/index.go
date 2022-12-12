package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/midbel/query/comma"
)

func main() {
	flag.Parse()

	var r io.Reader = os.Stdin
	if f, err := os.Open(flag.Arg(1)); err == nil {
		r = f
		defer f.Close()
	} else {
		if flag.Arg(1) != "" {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	if err := comma.Convert(r, os.Stdout, flag.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Fprintln(os.Stdout)
}
