package jsondiffprinter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/breml/jsondiffprinter/internal/jsonpatch"
	"github.com/breml/jsondiffprinter/internal/jsonpointer"
)

const (
	keyValueSeparatorJSON      = `: `
	keyValueSeparatorTerraform = ` = `

	keyQuoteJSON      = `"`
	keyQuoteTerraform = ``

	singleLineReplaceIndicatorTerraform = `~`

	singleLineReplaceTransitionIndicatorTerraform = `->`
)

// A Comparer compares two JSON documents and returns a JSON patch that
// transforms the first document into the second document.
type Comparer func(before, after any) ([]byte, error)

type Patch = jsonpatch.Patch

// A PatchSeriesPostProcessor processes the JSON patch series before the
// diff is printed. It can be used to modify the diff before it is printed.
type PatchSeriesPostProcessor func(diff Patch) Patch

// Formatter formats the diff if the given JSON patch is applied to the given
// JSON document.
type Formatter struct {
	w io.Writer
	c colorize

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
	patchSeriesPostProcess               PatchSeriesPostProcessor
}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter(w io.Writer, options ...Option) Formatter {
	f := Formatter{
		w: w,
		c: colorize{
			disable: true,
		},

		indentation:       "  ",
		commas:            true,
		keyValueSeparator: keyValueSeparatorJSON,
		keyQuote:          keyQuoteJSON,
	}

	for _, option := range options {
		option(&f)
	}

	return f
}

// NewTerraformFormatter creates a new Terraform formatter.
func NewTerraformFormatter(w io.Writer, options ...Option) Formatter {
	f := Formatter{
		w: w,
		c: colorize{
			disable: false,
		},

		indentation:                          "    ",
		indentedDiffMarkers:                  true,
		commas:                               false,
		keyValueSeparator:                    keyValueSeparatorTerraform,
		keyQuote:                             keyQuoteTerraform,
		singleLineReplace:                    true,
		singleLineReplaceIndicator:           singleLineReplaceIndicatorTerraform,
		singleLineReplaceTransitionIndicator: singleLineReplaceTransitionIndicatorTerraform,
		hideUnchanged:                        true,
		omitChangeIndicatorOnEmptyKey:        true,
	}

	for _, option := range options {
		option(&f)
	}

	return f
}

type valueType int

func (v valueType) leftBracket() string {
	switch v {
	case valueTypePlain:
		return ""
	case valueTypeObject:
		return "{"
	case valueTypeArray:
		return "["
	default:
		panic("undefined value type")
	}
}

func (v valueType) rightBracket() string {
	switch v {
	case valueTypePlain:
		return ""
	case valueTypeObject:
		return "}"
	case valueTypeArray:
		return "]"
	default:
		panic("undefined value type")
	}
}

const (
	valueTypePlain = iota
	valueTypeObject
	valueTypeArray
)

// Format writes the formatted representation of the jsonpatch applied to the
// provided original in pretty form.
//
// The argument original can either be of tye []byte or any of the JSON types:
// map[string]any, []any, bool, float64, string or nil.
// If an other type is passed, Format will return an error.
// If the type is []byte, the argument is treated as a marshaled JSON document
// and is unmarshaled before processing.
//
// The argument jsonpatch can either be of type []byte representing a JSON
// document following the JSON Patch specification (RFC 6902) or any type, that
// is marshalable to a JSON document following the before mentioned
// specification. In the second case is the argument marshaled to JSON before
// being processed.
func (f Formatter) Format(original any, jsonpatch any) error {
	beforePatchTestSeries, err := asPatchTestSeries(original, jsonpointer.NewPointer())
	if err != nil {
		return fmt.Errorf("failed to convert original JSON document to JSON patch series: %w", err)
	}
	patch, err := patchFromAny(jsonpatch)
	if err != nil {
		return fmt.Errorf("failed to process JSON patch: %w", err)
	}
	diff, err := compileDiffPatchSeries(beforePatchTestSeries, patch)
	if err != nil {
		return fmt.Errorf("failed to compile diff patch series: %w", err)
	}

	if f.patchSeriesPostProcess != nil {
		diff = f.patchSeriesPostProcess(diff)
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
		withKey := !currentPath.IsEmpty() || !f.omitChangeIndicatorOnEmptyKey

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
				f.printOp(printOpConfig{
					preDiffMarkerIndent: preDiffMarkerIndent,
					indent:              indent,
					key:                 currentKey,
					value:               buf.String(),
					valType:             valueTypeObject,
					op:                  op,
					withKey:             true,
				})

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
				f.printOp(printOpConfig{
					preDiffMarkerIndent: preDiffMarkerIndent,
					indent:              indent,
					key:                 currentKey,
					value:               buf.String(),
					valType:             valueTypeArray,
					op:                  op,
					withKey:             true,
				})

			default:
				if f.hideUnchanged && !isArray {
					unchangedAttributes++
					continue
				}

				v := f.formatIndent(op.Value, strings.Repeat(f.indentation, len(currentPath)), f.opTypeIndicator(op.Operation))
				f.printOp(printOpConfig{
					preDiffMarkerIndent: preDiffMarkerIndent,
					indent:              indent,
					key:                 currentKey,
					value:               v,
					op:                  op,
					withKey:             true,
				})
			}

		case jsonpatch.OperationAdd:
			hasChange = true
			v := f.formatIndent(op.Value, strings.Repeat(f.indentation, len(currentPath)), f.opTypeIndicator(op.Operation))
			f.printOp(printOpConfig{
				preDiffMarkerIndent: preDiffMarkerIndent,
				indent:              indent,
				key:                 currentKey,
				value:               v,
				op:                  op,
				withKey:             withKey,
			})

		case jsonpatch.OperationRemove:
			hasChange = true
			v := f.formatIndent(op.OldValue, strings.Repeat(f.indentation, len(currentPath)), f.opTypeIndicator(op.Operation))
			f.printOp(printOpConfig{
				preDiffMarkerIndent: preDiffMarkerIndent,
				indent:              indent,
				key:                 currentKey,
				value:               v,
				op:                  op,
				withKey:             withKey,
			})

		case jsonpatch.OperationReplace:
			hasChange = true

			if f.jsonInJSONComparer != nil {
				if ok := f.processJSONInJSON(op, currentPath, preDiffMarkerIndent, indent, currentKey); ok {
					continue
				}
			}

			vold := f.formatIndent(op.OldValue, strings.Repeat(f.indentation, len(currentPath)), f.opTypeIndicator(jsonpatch.OperationRemove))
			v := f.formatIndent(op.Value, strings.Repeat(f.indentation, len(currentPath)), f.opTypeIndicator(jsonpatch.OperationAdd))
			f.printOp(printOpConfig{
				preDiffMarkerIndent: preDiffMarkerIndent,
				indent:              indent,
				key:                 currentKey,
				value:               v,
				valueOld:            vold,
				valueOldComma:       f.printCommaOrNot(i, patch, op),
				op:                  op,
				withKey:             withKey,
			})
		}
		fmt.Fprintln(f.w, f.printCommaOrNot(i, patch, op))
	}

	if unchangedAttributes > 0 {
		unchanged := f.c.darkGrey(fmt.Sprintf("# (%d unchanged attribute hidden)", unchangedAttributes))
		fmt.Fprintf(f.w, "%s%s  %s\n", preDiffMarkerIndent, indent, unchanged)
	}

	return i, hasChange
}

type printOpConfig struct {
	preDiffMarkerIndent string
	indent              string
	key                 string
	value               string
	valueOld            string
	valueOldComma       string
	valType             valueType
	op                  jsonpatch.Operation
	withKey             bool
}

func (f Formatter) printOp(cfg printOpConfig) {
	if cfg.op.Operation == jsonpatch.OperationReplace && !f.singleLineReplace {
		op := cfg.op
		op.Operation = jsonpatch.OperationRemove
		f.printOp(printOpConfig{
			preDiffMarkerIndent: cfg.preDiffMarkerIndent,
			indent:              cfg.indent,
			key:                 cfg.key,
			value:               cfg.valueOld,
			valType:             cfg.valType,
			op:                  op,
			withKey:             cfg.withKey,
		})

		op = cfg.op
		op.Operation = jsonpatch.OperationAdd
		fmt.Fprintf(f.w, "%s\n", cfg.valueOldComma)
		f.printOp(printOpConfig{
			preDiffMarkerIndent: cfg.preDiffMarkerIndent,
			indent:              cfg.indent,
			key:                 cfg.key,
			value:               cfg.value,
			valType:             cfg.valType,
			op:                  op,
			withKey:             cfg.withKey,
		})
		return
	}

	keyNote := ""
	valueNote := ""
	if len(cfg.valType.leftBracket()) > 0 {
		keyNote = cfg.valType.leftBracket() + cfg.op.Metadata["note"] + "\n"
	} else {
		valueNote = cfg.op.Metadata["note"]
	}

	opTypeIndicator := f.opTypeIndicator(cfg.op.Operation)
	if cfg.op.Metadata["operationOverride"] != "" {
		opTypeIndicator = f.opTypeIndicator(jsonpatch.OperationType(cfg.op.Metadata["operationOverride"]))
	}

	if cfg.withKey {
		fmt.Fprintf(f.w, "%s%s %s%s%s", cfg.preDiffMarkerIndent, opTypeIndicator, cfg.indent, cfg.key, keyNote)
	} else {
		fmt.Fprint(f.w, "  ")
	}
	if cfg.valueOld != "" {
		fmt.Fprintf(f.w, "%s %s ", cfg.valueOld, f.c.yellow(f.singleLineReplaceTransitionIndicator))
	}
	fmt.Fprint(f.w, cfg.value, valueNote)
	if cfg.valType.rightBracket() != "" {
		fmt.Fprintf(f.w, "%s  %s%s", cfg.preDiffMarkerIndent, cfg.indent, cfg.valType.rightBracket())
	}
}

func (f Formatter) opTypeIndicator(opType jsonpatch.OperationType) string {
	switch opType {
	case jsonpatch.OperationTest:
		return " "
	case jsonpatch.OperationAdd:
		return f.c.green("+")
	case jsonpatch.OperationRemove:
		return f.c.red("-")
	case jsonpatch.OperationReplace:
		return f.c.yellow("~")
	default:
		panic("not supported operation type")
	}
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
	switch {
	case oldValueMapErr == nil && valueMapErr == nil:
		oldVal = oldValuejInjMap
		val = valuejInjMap
	case oldValueArrayErr == nil && valueArrayErr == nil:
		oldVal = oldValuejInjArray
		val = valuejInjArray
	default:
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

	withKey := !currentPath.IsEmpty() || !f.omitChangeIndicatorOnEmptyKey
	v := fmt.Sprintf("jsonencode(\n%s%s  %s%s%[1]s%[3]s  )", preDiffMarkerIndent, f.indentation, indent, strings.Trim(buf.String(), " "))
	opReplace := op
	opReplace.Operation = jsonpatch.OperationReplace
	f.printOp(printOpConfig{
		preDiffMarkerIndent: preDiffMarkerIndent,
		indent:              indent,
		key:                 currentKey,
		value:               v,
		op:                  opReplace,
		withKey:             withKey,
	})
	fmt.Fprint(f.w, "\n")

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
				sb.WriteString(" ")
			}
			sb.WriteString(f.prefix + prefix)
			sb.WriteString(f.indentation)
			if f.indentedDiffMarkers {
				sb.WriteString(operation)
				sb.WriteString(" ")
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
				sb.WriteString(" ")
			}
			sb.WriteString(f.prefix + prefix)
			sb.WriteString(f.indentation)
			if f.indentedDiffMarkers {
				sb.WriteString(operation)
				sb.WriteString(" ")
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
		var jsonEncode bool
		if str, ok := vt.(string); ok && f.jsonInJSONComparer != nil {
			var jInjMap map[string]any
			var jInjArray map[string]any

			mapErr := json.Unmarshal([]byte(str), &jInjMap)
			arrayErr := json.Unmarshal([]byte(str), &jInjArray)

			if mapErr == nil {
				vt = jInjMap
				jsonEncode = true
			}
			if arrayErr == nil {
				vt = jInjArray
				jsonEncode = true
			}
		}

		sb := strings.Builder{}
		encoder := json.NewEncoder(&sb)
		jsonInJSONPrefix := f.prefix + prefix + "  "
		if jsonEncode {
			jsonInJSONPrefix += f.indentation
			sb.WriteString("jsonencode(\n" + jsonInJSONPrefix)
		}
		encoder.SetIndent(jsonInJSONPrefix, f.indentation)
		encoder.SetEscapeHTML(false)
		err := encoder.Encode(vt)
		if err != nil {
			return fmt.Sprintf("<format error> %v%s%v", vt, f.keyValueSeparator, err)
		}
		if jsonEncode {
			sb.WriteString(f.prefix + prefix + "  " + ")")
		}

		return strings.Trim(sb.String(), " \n")
	}
}

const defaultPatchAllocationSize = 32

func asPatchTestSeries(value any, path jsonpointer.Pointer) (jsonpatch.Patch, error) {
	patches := make(jsonpatch.Patch, 0, defaultPatchAllocationSize)

	switch t := value.(type) {
	case []byte:
		if !path.IsEmpty() {
			return nil, fmt.Errorf("[]byte is only supported at root level in original JSON")
		}
		err := json.Unmarshal(t, &value)
		if err != nil {
			return nil, err
		}
		patches, err = asPatchTestSeries(value, path)
		if err != nil {
			return nil, err
		}
	case map[string]any:
		patches = append(patches, jsonpatch.Operation{
			Operation: jsonpatch.OperationTest,
			Path:      path,
			Value:     value,
		})

		for _, k := range keys(t) {
			ps, err := asPatchTestSeries(t[k], path.Append(k))
			if err != nil {
				return nil, err
			}
			patches = append(patches, ps...)
		}

	case []any:
		patches = append(patches, jsonpatch.Operation{
			Operation: jsonpatch.OperationTest,
			Path:      path,
			Value:     value,
		})

		for i, v := range t {
			ps, err := asPatchTestSeries(v, path.AppendIndex(i))
			if err != nil {
				return nil, err
			}
			patches = append(patches, ps...)
		}

	// All other types, that are used by encoding/json.Unmarshal to []any or map[string]any.
	case bool, float64, string, nil:
		patches = append(patches, jsonpatch.Operation{
			Operation: jsonpatch.OperationTest,
			Path:      path,
			Value:     value,
		})

	default:
		return nil, fmt.Errorf("unsupported type %T for original JSON", value)
	}

	return patches, nil
}

func compileDiffPatchSeries(src jsonpatch.Patch, patch jsonpatch.Patch) (jsonpatch.Patch, error) {
	var deletePath *jsonpointer.Pointer
	res := make(jsonpatch.Patch, 0, len(src)+len(patch))
	for opIndex := 0; opIndex < len(src); opIndex++ {
		op := src[opIndex]
		if deletePath != nil && deletePath.IsParentOf(op.Path) {
			continue
		}
		deletePath = nil

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
			opIndex--
			continue

		case jsonpatch.OperationReplace, jsonpatch.OperationRemove:
			// If the patch operation is a replace or delete operation, preserve the
			// old value and we mark all child operations for removal.
			patchop.OldValue = op.Value
			deletePath = &op.Path
		}

		res = append(res, patchop)

		if patchop.Operation == jsonpatch.OperationAdd {
			res = append(res, op)
		}

		if patchop.Operation == jsonpatch.OperationRemove && parentIsArray(src, patchop.Path) {
			for j := opIndex + 1; j < len(src); j++ {
				if src[j].Path.HasSameAncestorsAs(patchop.Path) {
					src[j].Path.DecrementIndex()
					continue
				}
				break
			}
		}
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

	sort.SliceStable(res, func(i, j int) bool {
		return res[i].Path.LessThan(res[j].Path)
	})

	return res, nil
}

func parentIsArray(patch jsonpatch.Patch, path jsonpointer.Pointer) bool {
	for i := range patch {
		if patch[i].Path.IsParentOf(path) {
			_, ok := patch[i].Value.([]any)
			return ok
		}
	}
	return false
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
