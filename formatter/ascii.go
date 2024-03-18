package formatter

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/wI2L/jsondiff"

	"github.com/breml/jsondiffprinter"
)

type AsciiFormatter struct {
	w io.Writer
}

func NewAsciiFormatter(w io.Writer) AsciiFormatter {
	return AsciiFormatter{w: w}
}

func (a AsciiFormatter) Format(patch jsondiff.Patch) error {
	_, err := a.printPatch(patch, nil, false)
	return err
}

func (a AsciiFormatter) printPatch(patch jsondiff.Patch, parentPath jsondiffprinter.Pointer, isArray bool) (int, error) {
	var i int
	for i = 0; i < len(patch); i++ {
		op := patch[i]
		currentPath, err := jsondiffprinter.NewPointerFromPath(op.Path)
		if err != nil {
			return 0, err
		}

		indent := strings.Repeat("  ", len(currentPath))

		if !currentPath.IsEmpty() && !parentPath.IsParentOf(currentPath) {
			break
		}

		currentKey := ""
		if !currentPath.IsEmpty() {
			if i > 0 {
				fmt.Fprintln(a.w, ",")
			}
			if !isArray {
				currentKey = fmt.Sprintf("%q: ", currentPath[len(currentPath)-1])
			}
		}

		switch op.Type {
		case jsondiff.OperationTest:
			switch vt := op.Value.(type) {
			case map[string]any:
				fmt.Fprintf(a.w, "  %s%s{\n", indent, currentKey)
				ii, err := a.printPatch(patch[i+1:], currentPath, false)
				if err != nil {
					return 0, err
				}
				i += ii
				fmt.Fprintf(a.w, "\n  %s%s", indent, "}")

			case []any:
				fmt.Fprintf(a.w, "  %s%s[\n", indent, currentKey)
				ii, err := a.printPatch(patch[i+1:], currentPath, true)
				if err != nil {
					return 0, err
				}
				i += ii
				fmt.Fprintf(a.w, "\n  %s%s", indent, "]")

			default:
				fmt.Fprintf(a.w, "  %s%s", indent, currentKey)
				v, err := json.MarshalIndent(vt, strings.Repeat("  ", len(currentPath)+1), "  ")
				if err != nil {
					return 0, err
				}
				fmt.Fprintf(a.w, "%s", v)
			}
		case jsondiff.OperationAdd:
			fmt.Fprintf(a.w, "+ %s%s", indent, currentKey)
			v, err := json.MarshalIndent(op.Value, strings.Repeat("  ", len(currentPath)+1), "  ")
			if err != nil {
				return 0, err
			}
			fmt.Fprintf(a.w, "%s", v)

		case jsondiff.OperationRemove:
			fmt.Fprintf(a.w, "- %s%s", indent, currentKey)
			v, err := json.MarshalIndent(op.OldValue, strings.Repeat("  ", len(currentPath)+1), "  ")
			if err != nil {
				return 0, err
			}
			fmt.Fprintf(a.w, "%s", v)

		case jsondiff.OperationReplace:
			fmt.Fprintf(a.w, "- %s%s", indent, currentKey)
			vold, err := json.MarshalIndent(op.OldValue, strings.Repeat("  ", len(currentPath)+1), "  ")
			if err != nil {
				return 0, err
			}
			fmt.Fprintf(a.w, "%s", vold)
			if i+1 < len(patch) && path.Dir(patch[i+1].Path) == path.Dir(op.Path) && len(strings.Split(patch[i+1].Path, "/")) == len(strings.Split(op.Path, "/")) {
				fmt.Fprint(a.w, ",")
			}
			fmt.Fprintln(a.w)
			fmt.Fprintf(a.w, "+ %s%s", indent, currentKey)
			v, err := json.MarshalIndent(op.Value, strings.Repeat("  ", len(currentPath)+1), "  ")
			if err != nil {
				return 0, err
			}
			fmt.Fprintf(a.w, "%s", v)
		}
	}

	if parentPath == nil {
		fmt.Fprintln(a.w)
	}

	return i, nil
}
