package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/breml/jsondiffprinter"
)

//go:embed source.json
var source []byte

//go:embed patch.json
var patch []byte

func main() {
	formatter := jsondiffprinter.NewJSONFormatter(os.Stdout)
	err := formatter.Format(source, patch)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
