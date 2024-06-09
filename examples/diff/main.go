package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/mattbaird/jsonpatch"

	"github.com/breml/jsondiffprinter"
)

//go:embed before.json
var before []byte

//go:embed after.json
var after []byte

func main() {
	patch, err := jsonpatch.CreatePatch(before, after)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = jsondiffprinter.Format(before, patch)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
