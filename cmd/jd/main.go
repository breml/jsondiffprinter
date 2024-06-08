package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	mianxiang "github.com/520MianXiangDuiXiang520/json-diff"
	victorlowther "github.com/VictorLowther/jsonpatch2"
	cameront "github.com/cameront/go-jsonpatch"
	herkyl "github.com/herkyl/patchwerk"
	mattbaird "github.com/mattbaird/jsonpatch"
	snorwin "github.com/snorwin/jsonpatch"
	wI2L "github.com/wI2L/jsondiff"

	"github.com/breml/jsondiffprinter"
)

const patchLibAcknowledgments = `
Acknowledgments for the supported JSON patch libraries:

* cameront: https://github.com/cameront/go-jsonpatch
* herkyl: https://github.com/herkyl/patchwerk
* mattbaird https://github.com/mattbaird/jsonpatch
* MianXiang (520MianXiangDuiXiang520): https://github.com/520MianXiangDuiXiang520/json-diff
* snorwin: https://github.com/snorwin/jsonpatch
* VictorLowther: https://github.com/VictorLowther/jsonpatch2
* wI2L: https://github.com/wI2L/jsondiff

`

func main() {
	if err := main0(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func main0(osArgs []string) error {
	app := App{}

	cliapp := &cli.App{
		Name:      "jd",
		Usage:     "Show the difference between two JSON files.",
		Args:      true,
		ArgsUsage: `before.json after.json`,
		Before: func(ctx *cli.Context) error {
			if ctx.NArg() != 2 {
				fmt.Fprintf(ctx.App.ErrWriter, "Error: missing arguments, usage: jd <before.json> <after.json>\n\n")
				cli.ShowAppHelpAndExit(ctx, 1)
			}
			return nil
		},
		HideHelpCommand: true,
		Action:          app.Run,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "format",
				Aliases:     []string{"f"},
				Usage:       `output format for the diff. Supported values: diff, terraform`,
				Value:       "diff",
				DefaultText: "diff",
				Destination: &app.format,
				Action: func(ctx *cli.Context, s string) error {
					switch strings.ToLower(s) {
					case "diff", "terraform":
						return nil
					default:
						return fmt.Errorf(`Flag "--format" value %q is not allowed, supported values: diff, terraform`, s)
					}
				},
			},
			&cli.StringFlag{
				Name:        "patchlib",
				Aliases:     []string{"p"},
				Usage:       `library, that is used to calculate the JSON patch between before and after. Supported values: cameront, herkyl, mattbaird, MianXiang, snorwin, VictorLowther, VictorLowther-paranoid, wI2L`,
				Value:       "mattbaird",
				DefaultText: "mattbaird",
				Destination: &app.patchLib,
				Action: func(ctx *cli.Context, s string) error {
					switch strings.ToLower(s) {
					case "cameront", "herkyl", "mattbaird", "mianxiang", "snorwin", "victorlowther", "victorlowther-paranoid", "wi2l":
						return nil
					default:
						return fmt.Errorf(`Flag "--patchlib" value %q is not allowed, supported values: cameront, herkyl, mattbaird, MianXiang, snorwin, VictorLowther, VictorLowther-paranoid, wI2L`, s)
					}
				},
			},
			&cli.BoolFlag{
				Name:        "json-in-json",
				Aliases:     []string{"j"},
				Usage:       "enable json-in-json processing (embeded json)",
				Destination: &app.jsonInJSON,
			},
			&cli.BoolFlag{
				Name:        "color",
				Aliases:     []string{"c"},
				Usage:       "enable colorful printing",
				Destination: &app.color,
			},
			&cli.BoolFlag{
				Name:        "hide-unchanged",
				Aliases:     []string{"u"},
				Usage:       "hide unchanged lines",
				Destination: &app.hideUnchanged,
			},
			&cli.BoolFlag{
				Name:        "show-patch",
				Usage:       "print the calculated patch.",
				Destination: &app.showPatch,
				Hidden:      true,
			},
		},
		CustomAppHelpTemplate: cli.AppHelpTemplate + patchLibAcknowledgments,
	}

	return cliapp.Run(osArgs)
}

type App struct {
	format        string
	patchLib      string
	jsonInJSON    bool
	color         bool
	hideUnchanged bool
	showPatch     bool
}

func (a *App) Run(ctx *cli.Context) error {
	beforeJSON, err := os.ReadFile(ctx.Args().Get(0))
	if err != nil {
		return fmt.Errorf("failed to read before.json: %w", err)
	}
	afterJSON, err := os.ReadFile(ctx.Args().Get(1))
	if err != nil {
		return fmt.Errorf("failed to read after.json: %w", err)
	}

	var before, after any
	err = json.Unmarshal(beforeJSON, &before)
	if err != nil {
		return fmt.Errorf("failed to unmarshal before.json: %w", err)
	}

	err = json.Unmarshal(afterJSON, &after)
	if err != nil {
		return fmt.Errorf("failed to unmarshal after.json: %w", err)
	}

	var patch any
	switch strings.ToLower(a.patchLib) {
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
		// TODO: add snorwin-threeway
		var patchList snorwin.JSONPatchList
		patchList, err = snorwin.CreateJSONPatch(after, before)
		patch = patchList.Raw()
	case "victorlowther":
		patch, err = victorlowther.Generate(beforeJSON, afterJSON, false)
	case "victorlowther-paranoid":
		patch, err = victorlowther.Generate(beforeJSON, afterJSON, true)
	case "wi2l":
		patch, err = wI2L.Compare(before, after)
	}
	if err != nil {
		return fmt.Errorf("failed to calculate JSON patch using %q: %w", a.patchLib, err)
	}

	a.printPatch(ctx, patch)

	options := []jsondiffprinter.Option{
		jsondiffprinter.WithColor(a.color),
		jsondiffprinter.WithHideUnchanged(a.hideUnchanged),
	}

	if a.jsonInJSON {
		options = append(options, jsondiffprinter.WithJSONinJSONCompare(a.compare))
	}

	switch strings.ToLower(a.format) {
	case "diff":
		err = jsondiffprinter.NewJSONFormatter(ctx.App.Writer, options...).Format(before, patch)
	case "terraform":
		err = jsondiffprinter.NewTerraformFormatter(ctx.App.Writer, options...).Format(before, patch)
	}
	if err != nil {
		return fmt.Errorf("failed to format using format %q: %w", a.format, err)
	}

	return nil
}

// TODO: combine with initial JSON Patch calculation in func Run.
func (a App) compare(before, after any) ([]byte, error) {
	var err error
	var beforeJSON, afterJSON []byte

	jsonMarshal := func() error {
		beforeJSON, err = json.Marshal(before)
		if err != nil {
			return err
		}
		afterJSON, err = json.Marshal(after)
		if err != nil {
			return err
		}
		return nil
	}

	var marshal bool
	var patch any
	switch strings.ToLower(a.patchLib) {
	case "cameront":
		patch, err = cameront.MakePatch(before, after)
		marshal = true
	case "herkyl":
		err = jsonMarshal()
		if err != nil {
			break
		}
		patch, err = herkyl.Diff(beforeJSON, afterJSON)
		marshal = true
	case "mattbaird":
		err = jsonMarshal()
		if err != nil {
			break
		}
		patch, err = mattbaird.CreatePatch(beforeJSON, afterJSON)
		marshal = true
	case "mianxiang":
		// TODO: consider options offered by 520MianXiangDuiXiang520/json-diff
		err = jsonMarshal()
		if err != nil {
			break
		}
		patch, err = mianxiang.AsDiffs(beforeJSON, afterJSON)
	case "snorwin":
		// TODO: add snorwin-threeway
		var patchList snorwin.JSONPatchList
		patchList, err = snorwin.CreateJSONPatch(after, before)
		patch = patchList.Raw()
	case "victorlowther":
		err = jsonMarshal()
		if err != nil {
			break
		}
		patch, err = victorlowther.Generate(beforeJSON, afterJSON, false)
		marshal = true
	case "victorlowther-paranoid":
		err = jsonMarshal()
		if err != nil {
			break
		}
		patch, err = victorlowther.Generate(beforeJSON, afterJSON, true)
		marshal = true
	case "wi2l":
		patch, err = wI2L.Compare(before, after)
		marshal = true
	}
	if err != nil {
		return nil, fmt.Errorf("failed to calculate JSON patch using %q: %w", a.patchLib, err)
	}

	var patchData []byte
	if marshal {
		buf := bytes.Buffer{}
		encoder := json.NewEncoder(&buf)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		err = encoder.Encode(patch)
		if err != nil {
			return nil, err
		}
		patchData = buf.Bytes()
	} else {
		patchData = patch.([]byte)
		patchData = append(patchData, '\n')
	}

	return patchData, nil
}

func (a App) printPatch(c *cli.Context, patch any) {
	if !a.showPatch {
		return
	}

	fmt.Fprintln(c.App.ErrWriter, "--- JSON patch ---")
	switch p := patch.(type) {
	case []byte:
		fmt.Fprintln(c.App.ErrWriter, string(p))
	default:
		patchJSON, err := json.MarshalIndent(patch, "", "  ")
		if err != nil {
			fmt.Fprintf(c.App.ErrWriter, "error: %v\n", err)
			return
		}

		fmt.Fprintln(c.App.ErrWriter, string(patchJSON))
	}
	fmt.Fprintln(c.App.ErrWriter, "--- JSON patch ---")
}
