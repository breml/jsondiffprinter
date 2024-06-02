//go:generate go run .

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	mianxiang "github.com/520MianXiangDuiXiang520/json-diff"
	victorlowther "github.com/VictorLowther/jsonpatch2"
	cameront "github.com/cameront/go-jsonpatch"
	herkyl "github.com/herkyl/patchwerk"
	mattbaird "github.com/mattbaird/jsonpatch"
	snorwin "github.com/snorwin/jsonpatch"
	wI2L "github.com/wI2L/jsondiff"

	"github.com/qri-io/jsonpointer"
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
	PatchLib   *string                   `json:"patchLib,omitempty"`
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

		beforeJSON := txtarchive.Files[0].Data
		afterJSON := txtarchive.Files[1].Data

		var before, after interface{}
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

		for i, pointer := range metadata.JSONInJSON {
			ptr, err := jsonpointer.Parse(pointer)
			die(err)

			beforeStr, err := ptr.Eval(before)
			die(err)

			afterStr, err := ptr.Eval(after)
			die(err)

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
	}
}

func compare(patchLib string, beforeJSON, afterJSON []byte) []byte {
	var before, after interface{}
	err := json.Unmarshal(beforeJSON, &before)
	die(err)

	err = json.Unmarshal(afterJSON, &after)
	die(err)

	var marshal bool
	var patch any
	switch strings.ToLower(patchLib) {
	case "cameront":
		patch, err = cameront.MakePatch(before, after)
		marshal = true
	case "herkyl":
		patch, err = herkyl.Diff(beforeJSON, afterJSON)
		marshal = true
	case "mattbaird":
		patch, err = mattbaird.CreatePatch(beforeJSON, afterJSON)
		marshal = true
	case "mianxiang":
		patch, err = mianxiang.AsDiffs(beforeJSON, afterJSON)
	case "snorwin":
		// TODO: add snorwin-threeway
		var patchList snorwin.JSONPatchList
		patchList, err = snorwin.CreateJSONPatch(after, before)
		patch = patchList.Raw()
	case "victorlowther":
		patch, err = victorlowther.Generate(beforeJSON, afterJSON, false)
		marshal = true
	case "victorlowther-paranoid":
		patch, err = victorlowther.Generate(beforeJSON, afterJSON, true)
		marshal = true
	case "wi2l":
		patch, err = wI2L.Compare(before, after)
		marshal = true
	default:
		fmt.Fprintf(os.Stderr, `Unknown patch lib %q, default to "wI2L"`, patchLib)
		patch, err = wI2L.Compare(before, after)
		marshal = true
	}
	die(err)

	var patchData []byte
	if marshal {
		buf := bytes.Buffer{}
		encoder := json.NewEncoder(&buf)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		err = encoder.Encode(patch)
		die(err)
		patchData = buf.Bytes()
	} else {
		patchData = patch.([]byte)
		patchData = append(patchData, '\n')
	}

	patchData = sortPatch(patchData)

	return append(patchData, '\n')
}

func sortPatch(in []byte) []byte {
	var patch wI2L.Patch
	err := json.Unmarshal(in, &patch)
	die(err)

	sort.SliceStable(patch, func(i, j int) bool {
		return patchLessThan(patch[i], patch[j])
	})

	in, err = json.MarshalIndent(patch, "", "  ")
	die(err)

	return in
}

func patchLessThan(a, b wI2L.Operation) bool {
	if a.Path == b.Path {
		return opOrder[a.Type] < opOrder[b.Type]
	}

	as := strings.Split(a.Path, "/")
	bs := strings.Split(b.Path, "/")

	for i := 0; i < min(len(as), len(bs)); i++ {
		if as[i] == "-" || bs[i] == "-" {
			return as[i] != "-"
		}
		if as[i] != bs[i] {
			return as[i] < bs[i]
		}
	}

	return len(as) < len(bs)
}

var opOrder = map[string]int{
	"test":    0,
	"replace": 1,
	"remove":  2,
	"add":     3,
}

func die(err error) {
	if err != nil {
		log.Panic(err)
	}
}
