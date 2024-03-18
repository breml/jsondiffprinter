package formatter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/wI2L/jsondiff"

	"github.com/breml/jsondiffprinter"
)

type TerraformFormatter struct {
	w io.Writer
}

func NewTerraformFormatter(w io.Writer) TerraformFormatter {
	return TerraformFormatter{w: w}
}

func (t TerraformFormatter) Format(patch jsondiff.Patch) error {
	_, err := t.printPatch(patch, nil, false)
	return err
}

func (t TerraformFormatter) printPatch(patch jsondiff.Patch, parentPath jsondiffprinter.Pointer, isArray bool) (int, error) {
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
				fmt.Fprintln(t.w)
			}
			if !isArray {
				currentKey = fmt.Sprintf("%s = ", currentPath[len(currentPath)-1])
			}
		}

		switch op.Type {
		case jsondiff.OperationTest:
			switch vt := op.Value.(type) {
			case map[string]any:
				fmt.Fprintf(t.w, "%s  %s{\n", indent, currentKey)
				ii, err := t.printPatch(patch[i+1:], currentPath, false)
				if err != nil {
					return 0, err
				}
				i += ii
				fmt.Fprintf(t.w, "\n%s  %s", indent, "}")

			case []any:
				fmt.Fprintf(t.w, "%s  %s[\n", indent, currentKey)
				ii, err := t.printPatch(patch[i+1:], currentPath, true)
				if err != nil {
					return 0, err
				}
				i += ii
				fmt.Fprintf(t.w, "\n%s  %s", indent, "]")

			default:
				fmt.Fprintf(t.w, "%s  %s", indent, currentKey)
				v, err := json.MarshalIndent(vt, strings.Repeat("  ", len(currentPath)+1), "  ")
				if err != nil {
					return 0, err
				}
				fmt.Fprintf(t.w, "%s", v)
			}
		case jsondiff.OperationAdd:
			fmt.Fprintf(t.w, "%s+ %s", indent, currentKey)
			v := t.FormatIndent(op.Value, strings.Repeat("  ", len(currentPath)+1), "  ")
			fmt.Fprintf(t.w, "%s", v)

		case jsondiff.OperationRemove:
			fmt.Fprintf(t.w, "%s- %s", indent, currentKey)
			v := t.FormatIndent(op.OldValue, strings.Repeat("  ", len(currentPath)+1), "  ")
			fmt.Fprintf(t.w, "%s", v)

		case jsondiff.OperationReplace:
			fmt.Fprintf(t.w, "%s~ %s", indent, currentKey)
			vold := t.FormatIndent(op.OldValue, strings.Repeat("  ", len(currentPath)+1), "  ")
			v := t.FormatIndent(op.Value, strings.Repeat("  ", len(currentPath)+1), "  ")
			fmt.Fprintf(t.w, "%s -> %s", vold, v)
		}
	}

	if parentPath == nil {
		fmt.Fprintln(t.w)
	}

	return i, nil
}

func (t TerraformFormatter) FormatIndent(v any, prefix string, indent string) string {
	switch vt := v.(type) {
	case map[string]any:
		sb := strings.Builder{}
		sb.WriteString("{\n")

		for k, v := range vt {
			sb.WriteString(prefix)
			sb.WriteString(indent)
			sb.WriteString(k)
			sb.WriteString(" = ")
			sb.WriteString(t.FormatIndent(v, prefix+indent, indent))
			sb.WriteString("\n")
		}

		sb.WriteString(prefix)
		sb.WriteString("}")

		return sb.String()

	case []any:
		sb := strings.Builder{}
		sb.WriteString("[\n")

		for _, v := range vt {
			sb.WriteString(prefix)
			sb.WriteString(indent)
			sb.WriteString(t.FormatIndent(v, prefix+indent, indent))
			sb.WriteString("\n")
		}

		sb.WriteString(prefix)
		sb.WriteString("]")

		return sb.String()

	default:
		v, err := json.MarshalIndent(vt, prefix, indent)
		if err != nil {
			return fmt.Sprintf("<format error> %v: %v", vt, err)
		}

		return string(v)
	}
}
