package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	MianXiangDuiXiang520 "github.com/520MianXiangDuiXiang520/json-diff"
	victorLowther "github.com/VictorLowther/jsonpatch2"
	cameront "github.com/cameront/go-jsonpatch"
	herkyl "github.com/herkyl/patchwerk"
	mattbaird "github.com/mattbaird/jsonpatch"
	snorwin "github.com/snorwin/jsonpatch"
	wI2L "github.com/wI2L/jsondiff"

	"github.com/breml/jsondiffprinter"
)

var (
	// Call it showPatch?
	debug    = flag.Bool("debug", false, "enable debug output")
	format   = flag.String("format", "ascii", "output format to use (ascii, terraform)")
	patchLib = flag.String("patchlib", "wI2L/jsondiff", "patch library to use (wI2L/jsondiff, mattbaird/jsonpatch, herkyl/patchwerk, snorwin/jsonpatch, VictorLowther/jsonpatch2, VictorLowther/jsonpatch2-paranoid, cameront/go-jsonpatch, 520MianXiangDuiXiang520/json-diff)")
)

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

	var patch any
	switch *patchLib {
	case "wI2L/jsondiff":
		patch, err = wI2L.Compare(before, after)
	case "mattbaird/jsonpatch":
		patch, err = mattbaird.CreatePatch(beforeJSON, afterJSON)
	case "herkyl/patchwerk":
		patch, err = herkyl.Diff(beforeJSON, afterJSON)
	case "snorwin/jsonpatch":
		var patchList snorwin.JSONPatchList
		patchList, err = snorwin.CreateJSONPatch(after, before)
		patch = patchList.Raw()
	case "VictorLowther/jsonpatch2":
		patch, err = victorLowther.Generate(beforeJSON, afterJSON, false)
	case "VictorLowther/jsonpatch2-paranoid":
		patch, err = victorLowther.Generate(beforeJSON, afterJSON, true)
	case "cameront/go-jsonpatch":
		patch, err = cameront.MakePatch(before, after)
	case "520MianXiangDuiXiang520/json-diff":
		// TODO: consider options offered by 520MianXiangDuiXiang520/json-diff
		patch, err = MianXiangDuiXiang520.AsDiffs(beforeJSON, afterJSON)
	}
	if err != nil {
		return err
	}

	printPatch(patch)

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

func printPatch(patch any) {
	if !*debug {
		return
	}

	switch p := patch.(type) {
	case []byte:
		fmt.Println(string(p))
	default:
		patchJSON, err := json.MarshalIndent(patch, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return
		}

		fmt.Println(string(patchJSON))
	}
}
