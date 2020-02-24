package main

import (
	"flag"
	"fmt"
	"os"

	imagecheck "github.com/delthas/image-check"
)

func main() {
	flag.Parse()
	path := flag.Arg(0)

	if path == "" {
		fmt.Fprintf(os.Stderr, "no file specified\n")
		os.Exit(128)
	}

	err := imagecheck.Check(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid file: %v\n", err.Error())
		os.Exit(1)
	}
}
