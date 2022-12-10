package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/query"
)

func main() {
	flag.Parse()

	err := query.Debug(os.Stdout, flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
