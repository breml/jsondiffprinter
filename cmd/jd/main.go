package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/wI2L/jsondiff"

	"github.com/breml/jsondiffprinter"
)

var format = flag.String("format", "ascii", "output format to use (ascii, terraform)")

func main() {
	flag.Parse()

	if len(flag.Args()) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <before.json> <after.json>\n", os.Args[0])
		os.Exit(1)
	}

	err := run(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	beforeJSON, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	afterJSON, err := os.ReadFile(args[1])
	if err != nil {
		return err
	}

	var before, after interface{}
	err = json.Unmarshal(beforeJSON, &before)
	if err != nil {
		return err
	}

	err = json.Unmarshal(afterJSON, &after)
	if err != nil {
		return err
	}

	patch, err := jsondiff.Compare(before, after)
	if err != nil {
		return err
	}

	switch *format {
	case "ascii":
		err = jsondiffprinter.NewJSONFormatter(os.Stdout).Format(before, patch)
	case "terraform":
		err = jsondiffprinter.NewTerraformFormatter(os.Stdout).Format(before, patch)
	default:
		return fmt.Errorf("unknown formatter: %s", *format)
	}

	return err
}
