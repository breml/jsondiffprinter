package jsondiffprinter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mitchellh/colorstring"

	"github.com/breml/jsondiffprinter/internal/jsonpatch"
	"github.com/breml/jsondiffprinter/internal/jsonpointer"
)

const (
	KeyValueSeparatorJSON      = `: `
	KeyValueSeparatorTerraform = ` = `

	KeyQuoteJSON      = `"`
	KeyQuoteTerraform = ``

	SingleLineReplaceIndicatorTerraform = `~`

	SingleLineReplaceTransitionIndicatorTerraform = `[yellow][bold]->[reset]`
)

type Comparer func(before, after any) ([]byte, error)

type Formatter struct {
	w io.Writer
	c colorstring.Colorize

	prefix                               string
	indentation                          string
	indentedDiffMarkers                  bool
	commas                               bool
	keyValueSeparator                    string
	keyQuote                             string
	singleLineReplace                    bool
	singleLineReplaceIndicator           string
	singleLineReplaceTransitionIndicator string
	hideUnchanged                        bool
	omitChangeIndicatorOnEmptyKey        bool
	jsonInJSONComparer                   Comparer
}

func NewJSONFormatter(w io.Writer, options ...Option) Formatter {
	f := Formatter{
		w: w,
		c: colorstring.Colorize{
			Colors:  colorstring.DefaultColors,
			Disable: true,
		},

		indentation:       "  ",
		commas:            true,
		keyValueSeparator: KeyValueSeparatorJSON,
		keyQuote:          KeyQuoteJSON,
	}

	for _, option := range options {
		option(&f)
	}

	return f
}

func NewTerraformFormatter(w io.Writer, options ...Option) Formatter {
	f := Formatter{
		w: w,
		c: colorstring.Colorize{
			Colors:  colorstring.DefaultColors,
			Disable: false,
		},

		indentation:                          "    ",
		indentedDiffMarkers:                  true,
		commas:                               false,
		keyValueSeparator:                    KeyValueSeparatorTerraform,
		keyQuote:                             KeyQuoteTerraform,
		singleLineReplace:                    true,
		singleLineReplaceIndicator:           SingleLineReplaceIndicatorTerraform,
		singleLineReplaceTransitionIndicator: SingleLineReplaceTransitionIndicatorTerraform,
		hideUnchanged:                        true,
		omitChangeIndicatorOnEmptyKey:        true,
	}

	for _, option := range options {
		option(&f)
	}

	return f
}

func (f Formatter) Format(before any, jsonpatch any) error {
	beforePatchTestSeries := asPatchTestSeries(before, jsonpointer.NewPointer())
	patch, err := patchFromAny(jsonpatch)
	if err != nil {
		return err
	}
	diff, err := compileDiffPatchSeries(beforePatchTestSeries, patch)
	if err != nil {
		return err
	}

	f.printPatch(diff, nil, false)
	return nil
}

func (f Formatter) printPatch(patch jsonpatch.Patch, parentPath jsonpointer.Pointer, isArray bool) (int, bool) {
	var i int
	var hasChange bool
	var unchangedAttributes int

	if len(patch) == 0 {
		return 0, false
	}

	var preDiffMarkerIndent, indent string
	if f.indentedDiffMarkers {
		preDiffMarkerIndent = f.prefix + strings.Repeat(f.indentation, len(patch[0].Path))
	} else {
		preDiffMarkerIndent = f.prefix
		indent = strings.Repeat(f.indentation, len(patch[0].Path))
	}

	for i = 0; i < len(patch); i++ {
		op := patch[i]
		currentPath := op.Path

		if !currentPath.IsEmpty() && !parentPath.IsParentOf(currentPath) {
			break
		}

		currentKey := ""
		if !currentPath.IsEmpty() && !isArray {
			currentKey = fmt.Sprintf("%s%s%s%s", f.keyQuote, currentPath[len(currentPath)-1], f.keyQuote, f.keyValueSeparator)
		}

		switch op.Operation {
		case jsonpatch.OperationTest:
			switch op.Value.(type) {
			case map[string]any:
				buf := &bytes.Buffer{}
				fNew := f
				fNew.w = buf

				ii, changed := fNew.printPatch(patch[i+1:], currentPath, false)
				i += ii

				if f.hideUnchanged && !changed {
					unchangedAttributes++
					continue
				}

				hasChange = true
				fmt.Fprintf(f.w, "%s  %s%s{\n", preDiffMarkerIndent, indent, currentKey)
				fmt.Fprint(f.w, buf.String())
				fmt.Fprintf(f.w, "%s  %s%s", preDiffMarkerIndent, indent, "}")

			case []any:
				buf := &bytes.Buffer{}
				fNew := f
				fNew.w = buf

				ii, changed := fNew.printPatch(patch[i+1:], currentPath, true)
				i += ii

				if f.hideUnchanged && !changed {
					unchangedAttributes++
					continue
				}

				hasChange = true
				fmt.Fprintf(f.w, "%s  %s%s[\n", preDiffMarkerIndent, indent, currentKey)
				fmt.Fprint(f.w, buf.String())
				fmt.Fprintf(f.w, "%s  %s%s", preDiffMarkerIndent, indent, "]")

			default:
				if f.hideUnchanged && !isArray {
					unchangedAttributes++
					continue
				}
				fmt.Fprintf(f.w, "%s  %s%s", preDiffMarkerIndent, indent, currentKey)
				v := f.formatIndent(op.Value, strings.Repeat(f.indentation, len(currentPath)), "  ")
				fmt.Fprintf(f.w, "%s", v)
			}
		case jsonpatch.OperationAdd:
			hasChange = true
			if !currentPath.IsEmpty() || !f.omitChangeIndicatorOnEmptyKey {
				fmt.Fprintf(f.w, f.c.Color("%s[bold][green]+[reset] %s%s"), preDiffMarkerIndent, indent, currentKey)
			}
			v := f.formatIndent(op.Value, strings.Repeat(f.indentation, len(currentPath)), f.c.Color("[bold][green]+[reset] "))
			fmt.Fprintf(f.w, "%s", v)

		case jsonpatch.OperationRemove:
			hasChange = true
			if !currentPath.IsEmpty() || !f.omitChangeIndicatorOnEmptyKey {
				fmt.Fprintf(f.w, f.c.Color("%s[bold][red]-[reset] %s%s"), preDiffMarkerIndent, indent, currentKey)
			}
			v := f.formatIndent(op.OldValue, strings.Repeat(f.indentation, len(currentPath)), f.c.Color("[bold][red]-[reset] "))
			fmt.Fprintf(f.w, "%s", v)

		case jsonpatch.OperationReplace:
			hasChange = true

			if f.jsonInJSONComparer != nil {
				if ok := f.processJSONInJSON(op, currentPath, preDiffMarkerIndent, indent, currentKey); ok {
					continue
				}
			}

			vold := f.formatIndent(op.OldValue, strings.Repeat(f.indentation, len(currentPath)), f.c.Color("[bold][red]-[reset] "))
			v := f.formatIndent(op.Value, strings.Repeat(f.indentation, len(currentPath)), f.c.Color("[bold][green]+[reset] "))

			if f.singleLineReplace {
				if !currentPath.IsEmpty() || !f.omitChangeIndicatorOnEmptyKey {
					fmt.Fprintf(f.w, f.c.Color("%s[bold][yellow]~[reset] %s%s"), preDiffMarkerIndent, indent, currentKey)
				}
				fmt.Fprintf(f.w, f.c.Color("%s %s %s\n"), vold, f.c.Color(f.singleLineReplaceTransitionIndicator), v)
				continue
			}

			if !currentPath.IsEmpty() || !f.omitChangeIndicatorOnEmptyKey {
				fmt.Fprintf(f.w, f.c.Color("%s[bold][red]-[reset] %s%s"), preDiffMarkerIndent, indent, currentKey)
			}
			fmt.Fprintf(f.w, "%s%s\n", vold, f.printCommaOrNot(i, patch, op))
			if !currentPath.IsEmpty() || !f.omitChangeIndicatorOnEmptyKey {
				fmt.Fprintf(f.w, f.c.Color("%s[bold][green]+[reset] %s%s"), preDiffMarkerIndent, indent, currentKey)
			}
			fmt.Fprintf(f.w, "%s", v)
		}
		fmt.Fprintln(f.w, f.printCommaOrNot(i, patch, op))
	}

	if unchangedAttributes > 0 {
		fmt.Fprintf(f.w, f.c.Color("%s%s  [dark_gray]# (%d unchanged attribute hidden)[reset]\n"), preDiffMarkerIndent, indent, unchangedAttributes)
	}

	return i, hasChange
}

func (f Formatter) processJSONInJSON(op jsonpatch.Operation, currentPath jsonpointer.Pointer, preDiffMarkerIndent, indent, currentKey string) bool {
	var oldValue, value string
	var ok bool

	if oldValue, ok = op.OldValue.(string); !ok {
		return false
	}
	if value, ok = op.Value.(string); !ok {
		return false
	}

	var oldValuejInjMap map[string]any
	var oldValuejInjArray []any
	var valuejInjMap map[string]any
	var valuejInjArray []any

	oldValueMapErr := json.Unmarshal([]byte(oldValue), &oldValuejInjMap)
	oldValueArrayErr := json.Unmarshal([]byte(oldValue), &oldValuejInjArray)
	valueMapErr := json.Unmarshal([]byte(value), &valuejInjMap)
	valueArrayErr := json.Unmarshal([]byte(value), &valuejInjArray)

	var oldVal, val any
	if oldValueMapErr == nil && valueMapErr == nil {
		oldVal = oldValuejInjMap
		val = valuejInjMap
	} else if oldValueArrayErr == nil && valueArrayErr == nil {
		oldVal = oldValuejInjArray
		val = valuejInjArray
	} else {
		return false
	}

	patch, err := f.jsonInJSONComparer(oldVal, val)
	if err != nil {
		return false
	}

	buf := &bytes.Buffer{}
	fNew := f
	fNew.w = buf
	fNew.prefix = preDiffMarkerIndent + f.indentation

	err = fNew.Format(oldVal, patch)
	if err != nil {
		return false
	}

	if !currentPath.IsEmpty() || !f.omitChangeIndicatorOnEmptyKey {
		fmt.Fprintf(f.w, f.c.Color("%s[bold][yellow]~[reset] %s%s"), preDiffMarkerIndent, indent, currentKey)
	}
	fmt.Fprint(f.w, "jsonencode(\n")
	fmt.Fprintf(f.w, "%s  %s%s", preDiffMarkerIndent+f.indentation, indent, bytes.Trim(buf.Bytes(), " "))
	fmt.Fprintf(f.w, "%s%s  )\n", preDiffMarkerIndent, indent)
	return true
}

// printCommaOrNot prints a comma if the next operation is in the same path.
func (f Formatter) printCommaOrNot(i int, patch jsonpatch.Patch, op jsonpatch.Operation) string {
	if !f.commas {
		return ""
	}
	// if there are no more operations, no comma is needed
	if i == len(patch)-1 {
		return ""
	}
	// if paths share the same ancestor, a comma is needed
	if patch[i+1].Path.HasSameAncestorsAs(op.Path) {
		return ","
	}
	return ""
}

func (f Formatter) formatIndent(v any, prefix string, operation string) string {
	switch vt := v.(type) {
	case map[string]any:
		sb := strings.Builder{}
		sb.WriteString("{\n")

		for i, k := range keys(vt) {
			v := vt[k]
			if !f.indentedDiffMarkers {
				sb.WriteString(operation)
			}
			sb.WriteString(f.prefix + prefix)
			sb.WriteString(f.indentation)
			if f.indentedDiffMarkers {
				sb.WriteString(operation)
			}
			sb.WriteString(f.keyQuote)
			sb.WriteString(k)
			sb.WriteString(f.keyQuote)
			sb.WriteString(f.keyValueSeparator)
			sb.WriteString(f.formatIndent(v, prefix+f.indentation, operation))
			if f.commas && i < len(vt)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}

		sb.WriteString(f.prefix + prefix + "  ")
		sb.WriteString("}")

		return sb.String()

	case []any:
		sb := strings.Builder{}
		sb.WriteString("[\n")

		for i, v := range vt {
			if !f.indentedDiffMarkers {
				sb.WriteString(operation)
			}
			sb.WriteString(f.prefix + prefix)
			sb.WriteString(f.indentation)
			if f.indentedDiffMarkers {
				sb.WriteString(operation)
			}
			sb.WriteString(f.formatIndent(v, prefix+f.indentation, operation))
			if f.commas && i < len(vt)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}

		sb.WriteString(f.prefix + prefix + "  ")
		sb.WriteString("]")

		return sb.String()

	default:
		if str, ok := vt.(string); ok && f.jsonInJSONComparer != nil {
			var jInjMap map[string]any
			var jInjArray map[string]any

			mapErr := json.Unmarshal([]byte(str), &jInjMap)
			arrayErr := json.Unmarshal([]byte(str), &jInjArray)

			if mapErr == nil {
				vt = jInjMap
			}
			if arrayErr == nil {
				vt = jInjArray
			}
		}
		sb := strings.Builder{}
		encoder := json.NewEncoder(&sb)
		encoder.SetIndent(f.prefix+prefix, f.indentation)
		encoder.SetEscapeHTML(false)
		err := encoder.Encode(vt)
		if err != nil {
			return fmt.Sprintf("<format error> %v%s%v", vt, f.keyValueSeparator, err)
		}

		return strings.Trim(sb.String(), " \n")
	}
}

const defaultPatchAllocationSize = 32

func asPatchTestSeries(value any, path jsonpointer.Pointer) jsonpatch.Patch {
	patches := make(jsonpatch.Patch, 0, defaultPatchAllocationSize)

	switch t := value.(type) {
	case map[string]any:
		patches = append(patches, jsonpatch.Operation{
			Operation: jsonpatch.OperationTest,
			Path:      path,
			Value:     value,
		})

		for _, k := range keys(t) {
			patches = append(patches, asPatchTestSeries(t[k], path.Append(k))...)
		}

	case []any:
		patches = append(patches, jsonpatch.Operation{
			Operation: jsonpatch.OperationTest,
			Path:      path,
			Value:     value,
		})

		for i, v := range t {
			patches = append(patches, asPatchTestSeries(v, path.AppendIndex(i))...)
		}

	// All other types, that are used by encoding/json.Unmarshal to []any or map[string]any.
	case bool, float64, string, nil:
		patches = append(patches, jsonpatch.Operation{
			Operation: jsonpatch.OperationTest,
			Path:      path,
			Value:     value,
		})

	default:
		panic(fmt.Sprintf("unsupported type %T", value))
	}

	return patches
}

func compileDiffPatchSeries(src jsonpatch.Patch, patch jsonpatch.Patch) (jsonpatch.Patch, error) {
	deletePath := jsonpointer.Pointer{}
	res := make(jsonpatch.Patch, 0, len(src)+len(patch))
	for _, op := range src {
		if !deletePath.IsEmpty() && deletePath.IsParentOf(op.Path) {
			continue
		}
		deletePath = jsonpointer.Pointer{}

		// Search patch for operation with the same path.
		// If none is found, keep the operation from the source document.
		i, ok := findPatchIndex(patch, op.Path)
		if !ok {
			res = append(res, op)
			continue
		}

		patchop := patch[i]
		patch = append(patch[:i], patch[i+1:]...)

		if patchop.Operation == jsonpatch.OperationAdd && patchop.Path.IsEmpty() {
			if len(patch) > 0 {
				return nil, fmt.Errorf("patch is not empty after it has been applied")
			}
			// If incomparable values are located at the root
			// of the document, an add operation to replace
			// the entire content of the document is provided.
			// https://tools.ietf.org/html/rfc6902#section-4.1
			//
			// We replace this operation based on the following rules:
			// * if op.Value is nil, return the add operation
			// * if patchop.Value is nil, return a remove operation
			// * else return a replace operation
			if op.Value == nil {
				return jsonpatch.Patch{
					patchop,
				}, nil
			}
			if patchop.Value == nil {
				return jsonpatch.Patch{
					jsonpatch.Operation{
						Operation: jsonpatch.OperationRemove,
						Path:      patchop.Path,
						OldValue:  op.Value,
					},
				}, nil
			}
			return jsonpatch.Patch{
				jsonpatch.Operation{
					Operation: jsonpatch.OperationReplace,
					Path:      patchop.Path,
					Value:     patchop.Value,
					OldValue:  op.Value,
				},
			}, nil
		}

		switch patchop.Operation {
		case jsonpatch.OperationTest:
			// If the patch operation is a test operation, skip it.
			continue

		case jsonpatch.OperationReplace, jsonpatch.OperationRemove:
			// If the patch operation is a replace or delete operation, preserve the
			// old value and we mark all child operations for removal.
			patchop.OldValue = op.Value
			deletePath = op.Path
		}

		res = append(res, patchop)
	}

	for i := 0; i < len(patch); i++ {
		if patch[i].Operation != jsonpatch.OperationAdd {
			continue
		}

		if patch[i].Operation == jsonpatch.OperationAdd {
			res = append(res, patch[i])
		}

		patch = append(patch[:i], patch[i+1:]...)
		i--
	}

	if len(patch) > 0 {
		return nil, fmt.Errorf("patch is not empty after it has been applied")
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Path.LessThan(res[j].Path)
	})

	return res, nil
}

func findPatchIndex(patch jsonpatch.Patch, path jsonpointer.Pointer) (int, bool) {
	for i := range patch {
		if patch[i].Path.Equals(path) {
			return i, true
		}
	}
	return 0, false
}

func keys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func patchFromAny(value any) (jsonpatch.Patch, error) {
	var jsonbody []byte
	var err error

	switch t := value.(type) {
	case []byte:
		jsonbody = t
	default:
		jsonbody, err = json.Marshal(value)
		if err != nil {
			return jsonpatch.Patch{}, err
		}
	}

	var patch jsonpatch.Patch
	err = json.Unmarshal(jsonbody, &patch)
	if err != nil {
		return jsonpatch.Patch{}, err
	}
	return patch, nil
}
