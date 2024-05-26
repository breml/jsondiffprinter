//go:generate go run .

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/qri-io/jsonpointer"
	"github.com/wI2L/jsondiff"
	"golang.org/x/tools/txtar"
)

const basePath = "../../testdata"

type metadata struct {
	JSON *struct {
		Indentation         *string `json:"indentation,omitempty"`
		IndentedDiffMarkers *bool   `json:"indentedDiffMarkers,omitempty"`
		Commas              *bool   `json:"commas,omitempty"`
		HideUnchanged       *bool   `json:"hideUnchanged,omitempty"`
	} `json:"json,omitempty"`
	Terraform *struct {
		Indentation   *string `json:"indentation,omitempty"`
		HideUnchanged *bool   `json:"hideUnchanged,omitempty"`
		MetadataAdder *bool   `json:"metadataAdder,omitempty"`
	} `json:"terraform,omitempty"`
	Metadata   map[string]map[string]any `json:"metadata,omitempty"`
	JSONInJSON []string                  `json:"jsonInJSON,omitempty"`
}

func main() {
	files, err := filepath.Glob(filepath.Join(basePath, "*.txtar"))
	die(err)

	for _, filename := range files {
		fmt.Println("Processing", filename)
		txtarchive, err := txtar.ParseFile(filename)
		die(err)

		if len(txtarchive.Comment) == 0 {
			txtarchive.Comment = []byte("{}")
		}

		var metadata metadata
		err = json.Unmarshal(txtarchive.Comment, &metadata)
		die(err)

		var before, after interface{}
		err = json.Unmarshal(txtarchive.Files[0].Data, &before)
		die(err)

		err = json.Unmarshal(txtarchive.Files[1].Data, &after)
		die(err)

		patch, err := jsondiff.Compare(before, after)
		die(err)

		buf := bytes.Buffer{}
		encoder := json.NewEncoder(&buf)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		err = encoder.Encode(patch)
		die(err)

		txtarchive.Files[1].Data = buf.Bytes()
		txtarchive.Files[1].Name = "patch.json"

		for i, pointer := range metadata.JSONInJSON {
			ptr, err := jsonpointer.Parse(pointer)
			die(err)

			beforeStr, err := ptr.Eval(before)
			die(err)

			afterStr, err := ptr.Eval(after)
			die(err)

			var before, after interface{}
			err = json.Unmarshal([]byte(beforeStr.(string)), &before)
			die(err)

			err = json.Unmarshal([]byte(afterStr.(string)), &after)
			die(err)

			patch, err := jsondiff.Compare(before, after)
			die(err)

			patchData, err := json.MarshalIndent(patch, "", "  ")
			die(err)

			patchFile := txtar.File{
				Name: fmt.Sprintf("jsonInJSON.%d.json", i),
				Data: append(patchData, '\n'),
			}

			txtarchive.Files = append(txtarchive.Files, patchFile)
		}

		metadata.JSONInJSON = nil
		txtarchive.Comment, err = json.MarshalIndent(metadata, "", "  ")
		die(err)

		buf2 := bytes.Buffer{}
		buf2.WriteString(string(txtarchive.Comment) + "\n")
		for _, f := range txtarchive.Files {
			buf2.WriteString("-- " + f.Name + " --\n")
			buf2.WriteString(string(f.Data))
		}

		targetFilename := filepath.Join(basePath, "generated", filepath.Base(filename))
		err = os.WriteFile(targetFilename, buf2.Bytes(), 0o644)
		die(err)
	}
}

func die(err error) {
	if err != nil {
		log.Panic(err)
	}
}
