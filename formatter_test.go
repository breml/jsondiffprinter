package jsondiffprinter_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/breml/jsondiffprinter"
	"github.com/breml/jsondiffprinter/internal/jsonpatch"
	"github.com/breml/jsondiffprinter/internal/require"
)

type metadata struct {
	JSON struct {
		Indentation         *string `json:"indentation"`
		IndentedDiffMarkers *bool   `json:"indentedDiffMarkers"`
		Commas              *bool   `json:"commas"`
		HideUnchanged       *bool   `json:"hideUnchanged"`
	} `json:"json"`
	Terraform struct {
		Indentation   *string `json:"indentation"`
		HideUnchanged *bool   `json:"hideUnchanged"`
		NoteAdder     *bool   `json:"noteAdder"`
	} `json:"terraform"`
}

func TestFormatter(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "generated", "*.txtar"))
	require.NoError(t, err)

	for _, filename := range files {
		filename := filename
		t.Run(filename, func(t *testing.T) {
			txtar, err := txtar.ParseFile(filename)
			require.NoError(t, err)

			var before interface{}
			err = json.Unmarshal(txtarFileByName(t, txtar, "before.json").Data, &before)
			require.NoError(t, err)

			jsonInJSONInvocation := 0
			jsonInJSONCompare := func(before, after any) ([]byte, error) {
				defer func() {
					jsonInJSONInvocation++
				}()

				return txtarFileByName(t, txtar, fmt.Sprintf("jsonInJSON.%d.json", jsonInJSONInvocation)).Data, nil
			}

			if len(txtar.Comment) == 0 {
				txtar.Comment = []byte("{}")
			}

			var metadata metadata
			err = json.Unmarshal(txtar.Comment, &metadata)
			require.NoError(t, err)

			jsonOptions := append([]jsondiffprinter.Option{},
				jsondiffprinter.WithColor(false),
				jsondiffprinter.WithIndentation("  "),
			)

			terraformOptions := append([]jsondiffprinter.Option{},
				jsondiffprinter.WithColor(false),
				jsondiffprinter.WithIndentation("  "),
				jsondiffprinter.WithHideUnchanged(true),
				jsondiffprinter.WithJSONinJSONCompare(jsonInJSONCompare),
			)

			if metadata.JSON.Indentation != nil {
				jsonOptions = append(jsonOptions, jsondiffprinter.WithIndentation(*metadata.JSON.Indentation))
			}
			if metadata.JSON.IndentedDiffMarkers != nil {
				jsonOptions = append(jsonOptions, jsondiffprinter.WithIndentedDiffMarkers(*metadata.JSON.IndentedDiffMarkers))
			}

			if metadata.JSON.Commas != nil {
				jsonOptions = append(jsonOptions, jsondiffprinter.WithCommas(*metadata.JSON.Commas))
			}

			if metadata.JSON.HideUnchanged != nil {
				jsonOptions = append(jsonOptions, jsondiffprinter.WithHideUnchanged(*metadata.JSON.HideUnchanged))
			}

			if metadata.Terraform.Indentation != nil {
				terraformOptions = append(terraformOptions, jsondiffprinter.WithIndentation(*metadata.Terraform.Indentation))
			}

			if metadata.Terraform.HideUnchanged != nil {
				terraformOptions = append(terraformOptions, jsondiffprinter.WithHideUnchanged(*metadata.Terraform.HideUnchanged))
			}

			if metadata.Terraform.NoteAdder != nil {
				terraformOptions = append(terraformOptions, jsondiffprinter.WithPatchSeriesPostProcess(noteAdder))
			}

			var buf bytes.Buffer
			formatters := []struct {
				name      string
				formatter jsondiffprinter.Formatter

				wantFilename string
			}{
				{
					name:         "json",
					formatter:    jsondiffprinter.NewJSONFormatter(&buf, jsonOptions...),
					wantFilename: "diff.json",
				},
				{
					name:         "terraform",
					formatter:    jsondiffprinter.NewTerraformFormatter(&buf, terraformOptions...),
					wantFilename: "diff.tf",
				},
			}

			for _, formatter := range formatters {
				formatter := formatter
				t.Run(formatter.name, func(t *testing.T) {
					jsonInJSONInvocation = 0
					buf.Reset()

					err := formatter.formatter.Format(before, txtarFileByName(t, txtar, "patch.json").Data)
					require.NoError(t, err)

					require.Equal(t, string(txtarFileByName(t, txtar, formatter.wantFilename).Data), buf.String())
				})
			}
		})
	}
}

func txtarFileByName(t *testing.T, txtar *txtar.Archive, name string) *txtar.File {
	t.Helper()

	for _, f := range txtar.Files {
		if f.Name == name {
			return &f
		}
	}

	t.Fatalf("file %q not found", name)
	return nil
}

// FIXME: make this generic, such that a structure `replace_paths`
// from the Terraform plan.json can be provided.
func noteAdder(diff jsonpatch.Patch) jsonpatch.Patch {
	diff[2].OperationOverride = jsonpatch.OperationReplace
	diff[2].Note = "forces replacement"

	return diff
}
