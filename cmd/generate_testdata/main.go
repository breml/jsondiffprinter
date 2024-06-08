//go:generate go run .

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc64"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	mianxiang "github.com/520MianXiangDuiXiang520/json-diff"
	victorlowther "github.com/VictorLowther/jsonpatch2"
	cameront "github.com/cameront/go-jsonpatch"
	herkyl "github.com/herkyl/patchwerk"
	mattbaird "github.com/mattbaird/jsonpatch"
	"github.com/qri-io/jsonpointer"
	snorwin "github.com/snorwin/jsonpatch"
	wI2L "github.com/wI2L/jsondiff"
	"golang.org/x/tools/txtar"
)

const basePath = "../../testdata"

type metadata struct {
	JSON *struct {
		Indentation         *string `json:"indentation,omitempty"`
		IndentedDiffMarkers *bool   `json:"indentedDiffMarkers,omitempty"`
		Commas              *bool   `json:"commas,omitempty"`
		HideUnchanged       *bool   `json:"hideUnchanged,omitempty"`
		JSONInJSON          *bool   `json:"jsonInJSON,omitempty"`
	} `json:"json,omitempty"`
	Terraform *struct {
		Indentation   *string `json:"indentation,omitempty"`
		HideUnchanged *bool   `json:"hideUnchanged,omitempty"`
		MetadataAdder *bool   `json:"metadataAdder,omitempty"`
		JSONInJSON    *bool   `json:"jsonInJSON,omitempty"`
	} `json:"terraform,omitempty"`
	Metadata   map[string]map[string]any `json:"metadata,omitempty"`
	JSONInJSON []string                  `json:"jsonInJSON,omitempty"`
	PatchLib   *string                   `json:"patchLib,omitempty"`
}

type State map[string]checksum

type checksum struct {
	Checksum uint64 `json:"checksum,string"`
}

func main() {
	s, err := os.ReadFile(filepath.Join(basePath, "generated", ".state"))
	die(err)

	var currentState State
	err = json.Unmarshal(s, &currentState)
	die(err)

	h := crc64.New(crc64.MakeTable(crc64.ECMA))

	files, err := filepath.Glob(filepath.Join(basePath, "*.txtar"))
	die(err)
	newState := make(State, len(files))
	for _, filename := range files {
		fmt.Print("Processing", filename, "...")

		fbody, err := os.ReadFile(filename)
		die(err)

		h.Reset()
		h.Write(fbody)
		if currentState[filename].Checksum == h.Sum64() {
			fmt.Println("unchanged")
			newState[filename] = currentState[filename]
			delete(currentState, filename)
			continue
		}

		txtarchive, err := txtar.ParseFile(filename)
		die(err)

		if len(txtarchive.Comment) == 0 {
			txtarchive.Comment = []byte("{}")
		}

		var metadata metadata
		err = json.Unmarshal(txtarchive.Comment, &metadata)
		die(err)

		beforeJSON := txtarchive.Files[0].Data
		afterJSON := txtarchive.Files[1].Data

		var before, after any
		err = json.Unmarshal(beforeJSON, &before)
		die(err)

		err = json.Unmarshal(afterJSON, &after)
		die(err)

		patchLib := "wI2L"
		if metadata.PatchLib != nil {
			patchLib = *metadata.PatchLib
		}

		txtarchive.Files[1].Data = compare(patchLib, beforeJSON, afterJSON)
		txtarchive.Files[1].Name = "patch.json"

		var patch wI2L.Patch
		err = json.Unmarshal(txtarchive.Files[1].Data, &patch)
		die(err)

		orderedJSONInJSON := make([]string, 0, len(metadata.JSONInJSON))
		for _, op := range patch {
			if slices.Contains(metadata.JSONInJSON, op.Path) {
				orderedJSONInJSON = append(orderedJSONInJSON, op.Path)
			}
		}

		for i, pointer := range orderedJSONInJSON {
			ptr, err := jsonpointer.Parse(pointer)
			die(err)

			beforeStr, err := ptr.Eval(before)
			die(err)

			if beforeStr == nil {
				beforeStr = "null"
			}

			afterStr, err := ptr.Eval(after)
			die(err)

			if afterStr == nil {
				afterStr = "null"
			}

			patchData := compare(patchLib, []byte(beforeStr.(string)), []byte(afterStr.(string)))

			patchFile := txtar.File{
				Name: fmt.Sprintf("jsonInJSON.%d.json", i),
				Data: patchData,
			}

			txtarchive.Files = append(txtarchive.Files, patchFile)
		}

		metadata.JSONInJSON = nil
		metadata.PatchLib = nil
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

		fbody, err = os.ReadFile(filename)
		die(err)

		h.Reset()
		h.Write(fbody)
		newState[filename] = checksum{h.Sum64()}
		delete(currentState, filename)

		fmt.Println("done")
	}

	for filename := range currentState {
		fmt.Print("Remove", filename, "...")
		err = os.Remove(strings.Replace(filename, "testdata", filepath.Join("testdata", "generated"), 1))
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println("done")
	}

	s, err = json.MarshalIndent(newState, "", "  ")
	die(err)

	err = os.WriteFile(filepath.Join(basePath, "generated", ".state"), s, 0o660)
	die(err)
}

func compare(patchLib string, beforeJSON, afterJSON []byte) []byte {
	var before, after any
	err := json.Unmarshal(beforeJSON, &before)
	die(err)

	err = json.Unmarshal(afterJSON, &after)
	die(err)

	var unmarshal bool
	var patch any
	switch strings.ToLower(patchLib) {
	case "cameront":
		patch, err = cameront.MakePatch(before, after)
	case "herkyl":
		patch, err = herkyl.Diff(beforeJSON, afterJSON)
	case "mattbaird":
		patch, err = mattbaird.CreatePatch(beforeJSON, afterJSON)
	case "mianxiang":
		patch, err = mianxiang.AsDiffs(beforeJSON, afterJSON)
		unmarshal = true
	case "snorwin":
		// TODO: add snorwin-threeway
		var patchList snorwin.JSONPatchList
		patchList, err = snorwin.CreateJSONPatch(after, before)
		patch = patchList.Raw()
		unmarshal = true
	case "victorlowther":
		patch, err = victorlowther.Generate(beforeJSON, afterJSON, false)
	case "victorlowther-paranoid":
		patch, err = victorlowther.Generate(beforeJSON, afterJSON, true)
	case "wi2l":
		patch, err = wI2L.Compare(before, after)
	default:
		fmt.Fprintf(os.Stderr, `Unknown patch lib %q, default to "wI2L"`, patchLib)
		patch, err = wI2L.Compare(before, after)
	}
	die(err)

	if unmarshal {
		var t wI2L.Patch
		err = json.Unmarshal(patch.([]byte), &t)
		die(err)
		patch = t
	}

	buf := bytes.Buffer{}
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(patch)
	die(err)

	return buf.Bytes()
}

func die(err error) {
	if err != nil {
		log.Panic(err)
	}
}
