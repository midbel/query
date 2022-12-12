package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/query/comma"
)

func main() {
	flag.Parse()

	ix, err := comma.Parse(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Printf("%#v\n", ix)
}
