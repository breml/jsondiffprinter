package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	mianxiang "github.com/520MianXiangDuiXiang520/json-diff"
	victorlowther "github.com/VictorLowther/jsonpatch2"
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
	patchLib = flag.String("patchlib", "wI2L", "patch library to use (cameront, herkyl, mattbaird, MianXiang, snorwin, VictorLowther, VictorLowther-paranoid, wI2L)")
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
	switch strings.ToLower(*patchLib) {
	case "cameront":
		patch, err = cameront.MakePatch(before, after)
	case "herkyl":
		patch, err = herkyl.Diff(beforeJSON, afterJSON)
	case "mattbaird":
		patch, err = mattbaird.CreatePatch(beforeJSON, afterJSON)
	case "mianxiang":
		// TODO: consider options offered by 520MianXiangDuiXiang520/json-diff
		patch, err = mianxiang.AsDiffs(beforeJSON, afterJSON)
	case "snorwin":
		var patchList snorwin.JSONPatchList
		patchList, err = snorwin.CreateJSONPatch(after, before)
		patch = patchList.Raw()
	case "victorlowther":
		patch, err = victorlowther.Generate(beforeJSON, afterJSON, false)
	case "victorlowther-paranoid":
		patch, err = victorlowther.Generate(beforeJSON, afterJSON, true)
	default: // "wI2L"
		patch, err = wI2L.Compare(before, after)
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
