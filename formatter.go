package jsondiffprinter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
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

type notePosition int

const (
	notePositionKey notePosition = iota
	notePositionValue
	notePositionEnd
)

func (v valueType) notePos() notePosition {
	switch v {
	case valueTypePlain:
		return notePositionValue
	case valueTypeObject:
		return notePositionKey
	case valueTypeJSONinJSONObject:
		return notePositionEnd
	case valueTypeArray:
		return notePositionKey
	case valueTypeJSONinJSONArray:
		return notePositionEnd
	default:
		panic("undefined value type")
	}
}

func (v valueType) leftBracket() string {
	switch v {
	case valueTypePlain:
		return ""
	case valueTypeObject:
		return "{"
	case valueTypeJSONinJSONObject:
		return fmt.Sprintf("jsonencode(")
	case valueTypeArray:
		return "["
	case valueTypeJSONinJSONArray:
		return fmt.Sprintf("jsonencode(")
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
	case valueTypeJSONinJSONObject:
		return fmt.Sprintf(")")
	case valueTypeArray:
		return "]"
	case valueTypeJSONinJSONArray:
		return fmt.Sprintf(")")
	default:
		panic("undefined value type")
	}
}

const (
	valueTypePlain valueType = iota
	valueTypeObject
	valueTypeJSONinJSONObject
	valueTypeArray
	valueTypeJSONinJSONArray
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
	originalPatchTestSeries, err := f.asPatchTestSeries(original, jsonpointer.NewPointer())
	if err != nil {
		return fmt.Errorf("failed to convert JSON document to JSON patch series: %w", err)
	}
	patch, err := f.patchFromAny(jsonpatch)
	if err != nil {
		return fmt.Errorf("failed to process JSON patch: %w", err)
	}
	diff, err := f.compileDiffPatchSeries(originalPatchTestSeries, patch)
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
		op := patch[i].Clone()
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
			case jsonInJSONObject:
				buf := &bytes.Buffer{}
				fNew := f
				fNew.w = buf
				fNew.prefix += fNew.indentation

				endIndex := i + 1
				for ; endIndex < len(patch); endIndex++ {
					if !currentPath.IsAncestorOf(patch[endIndex].Path) {
						break
					}
				}

				patch[i].Value = map[string]any(patch[i].Value.(jsonInJSONObject))
				delete(patch[i].Metadata, "note")
				ii, changed := fNew.printPatch(patch[i:endIndex], currentPath[:max(0, len(currentPath)-1)], true)
				i += ii - 1

				if f.hideUnchanged && !changed {
					unchangedAttributes++
					continue
				}

				if len(currentPath) > 0 {
					op.Operation = jsonpatch.OperationReplace
				}
				hasChange = true
				f.printOp(printOpConfig{
					preDiffMarkerIndent: preDiffMarkerIndent,
					indent:              indent,
					key:                 currentKey,
					value:               buf.String(),
					valType:             valueTypeJSONinJSONObject,
					op:                  op,
					withKey:             true,
				})

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

			case jsonInJSONArray:
				buf := &bytes.Buffer{}
				fNew := f
				fNew.w = buf
				fNew.prefix += fNew.indentation

				endIndex := i + 1
				for ; endIndex < len(patch); endIndex++ {
					if !currentPath.IsAncestorOf(patch[endIndex].Path) {
						break
					}
				}

				patch[i].Value = []any(patch[i].Value.(jsonInJSONArray))
				delete(patch[i].Metadata, "note")
				ii, changed := fNew.printPatch(patch[i:endIndex], currentPath[:max(0, len(currentPath)-1)], true)
				i += ii - 1

				if f.hideUnchanged && !changed {
					unchangedAttributes++
					continue
				}

				if len(currentPath) > 0 {
					op.Operation = jsonpatch.OperationReplace
				}
				hasChange = true
				f.printOp(printOpConfig{
					preDiffMarkerIndent: preDiffMarkerIndent,
					indent:              indent,
					key:                 currentKey,
					value:               buf.String(),
					valType:             valueTypeJSONinJSONArray,
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

	leftBracket := ""
	valueNote := ""
	endNote := ""
	if len(cfg.valType.leftBracket()) > 0 {
		leftBracket = cfg.valType.leftBracket() + "\n"
	}
	switch cfg.valType.notePos() {
	case notePositionKey:
		leftBracket = cfg.valType.leftBracket() + cfg.op.Metadata["note"] + "\n"
	case notePositionValue:
		valueNote = cfg.op.Metadata["note"]
	case notePositionEnd:
		endNote = cfg.op.Metadata["note"]
	}

	opTypeIndicator := f.opTypeIndicator(cfg.op.Operation)
	if cfg.op.Metadata["operationOverride"] != "" {
		opTypeIndicator = f.opTypeIndicator(jsonpatch.OperationType(cfg.op.Metadata["operationOverride"]))
	}

	if cfg.withKey {
		fmt.Fprintf(f.w, "%s%s %s%s%s", cfg.preDiffMarkerIndent, opTypeIndicator, cfg.indent, cfg.key, leftBracket)
	} else {
		fmt.Fprint(f.w, "  ")
	}
	if cfg.valueOld != "" {
		fmt.Fprintf(f.w, "%s %s ", cfg.valueOld, f.c.yellow(f.singleLineReplaceTransitionIndicator))
	}
	fmt.Fprint(f.w, cfg.value, valueNote)
	if cfg.valType.rightBracket() != "" {
		fmt.Fprintf(f.w, "%s  %s%s%s", cfg.preDiffMarkerIndent, cfg.indent, cfg.valType.rightBracket(), endNote)
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
	case jsonInJSONObject:
		sb := strings.Builder{}
		sb.WriteString("jsonencode(\n")
		sb.WriteString(f.prefix + prefix + f.indentation + "  ")
		sb.WriteString(f.formatIndent(map[string]any(vt), prefix+f.indentation, operation))
		sb.WriteString("\n" + f.prefix + prefix + "  " + ")")
		return sb.String()
	case jsonInJSONArray:
		sb := strings.Builder{}
		sb.WriteString("jsonencode(\n")
		sb.WriteString(f.prefix + prefix + f.indentation + "  ")
		sb.WriteString(f.formatIndent([]any(vt), prefix+f.indentation, operation))
		sb.WriteString("\n" + f.prefix + prefix + "  " + ")")
		return sb.String()
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
		sb := strings.Builder{}
		encoder := json.NewEncoder(&sb)
		encoder.SetIndent(f.prefix+prefix+"  ", f.indentation)
		encoder.SetEscapeHTML(false)
		err := encoder.Encode(vt)
		if err != nil {
			return fmt.Sprintf("<format error> %v%s%v", vt, f.keyValueSeparator, err)
		}

		return strings.Trim(sb.String(), " \n")
	}
}

const defaultPatchAllocationSize = 32

func (f Formatter) asPatchTestSeries(inValue any, path jsonpointer.Pointer) (jsonpatch.Patch, error) {
	patches := make(jsonpatch.Patch, 0, defaultPatchAllocationSize)

	value := inValue
	if v, ok := value.(jsonInJSONObject); ok {
		value = map[string]any(v)
	}
	if v, ok := value.(jsonInJSONArray); ok {
		value = []any(v)
	}

	switch t := value.(type) {
	case []byte:
		if !path.IsEmpty() {
			return nil, fmt.Errorf("[]byte is only supported at root level in original JSON")
		}
		err := json.Unmarshal(t, &value)
		if err != nil {
			return nil, err
		}
		patches, err = f.asPatchTestSeries(value, path)
		if err != nil {
			return nil, err
		}

	case map[string]any:
		patches = append(patches, jsonpatch.Operation{
			Operation: jsonpatch.OperationTest,
			Path:      path,
			Value:     inValue,
		})

		for _, k := range keys(t) {
			ps, err := f.asPatchTestSeries(t[k], path.AppendKey(k))
			if err != nil {
				return nil, err
			}
			patches = append(patches, ps...)
		}

	case []any:
		patches = append(patches, jsonpatch.Operation{
			Operation: jsonpatch.OperationTest,
			Path:      path,
			Value:     inValue,
		})

		for i, v := range t {
			ps, err := f.asPatchTestSeries(v, path.AppendIndex(i))
			if err != nil {
				return nil, err
			}
			patches = append(patches, ps...)
		}

	// All other types, that are used by encoding/json.Unmarshal to []any or map[string]any.
	case bool, float64, nil:
		patches = append(patches, jsonpatch.Operation{
			Operation: jsonpatch.OperationTest,
			Path:      path,
			Value:     value,
		})

	case string:
		var unmarshaledValue any

		if f.jsonInJSONComparer != nil {
			if jsonValue, ok := asJSONInJSON(t); ok {
				unmarshaledValue = jsonValue
			}
		}

		patches = append(patches, jsonpatch.Operation{
			Operation:        jsonpatch.OperationTest,
			Path:             path,
			Value:            value,
			UnmarshaledValue: unmarshaledValue,
		})

	default:
		return nil, fmt.Errorf("unsupported type %T for original JSON", value)
	}

	return patches, nil
}

type (
	jsonInJSONObject map[string]any
	jsonInJSONArray  []any
)

func asJSONInJSON(v any) (any, bool) {
	value, ok := v.(string)
	if !ok {
		return nil, false
	}

	var valuejInjMap map[string]any
	valueMapErr := json.Unmarshal([]byte(value), &valuejInjMap)
	if valueMapErr == nil {
		return jsonInJSONObject(valuejInjMap), true
	}

	var valuejInjArray []any
	valueArrayErr := json.Unmarshal([]byte(value), &valuejInjArray)
	if valueArrayErr == nil {
		return jsonInJSONArray(valuejInjArray), true
	}

	return nil, false
}

func (f Formatter) compileDiffPatchSeries(src jsonpatch.Patch, patch jsonpatch.Patch) (jsonpatch.Patch, error) {
	if len(src) == 0 {
		src = jsonpatch.Patch{}
	}

	for opIndex := 0; opIndex < len(patch); opIndex++ {
		patchOp := patch[opIndex]
		switch patchOp.Operation {
		case jsonpatch.OperationAdd:
			if f.jsonInJSONComparer != nil && patchOp.UnmarshaledValue != nil {
				patchOp.Value = patchOp.UnmarshaledValue
			}

			if patchOp.Path.IsEmpty() {
				op := jsonpatch.Operation{}
				if len(src) > 0 {
					op = src[0]
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
					src = jsonpatch.Patch{
						patchOp,
					}
					break
				}
				if patchOp.Value == nil {
					src = jsonpatch.Patch{
						jsonpatch.Operation{
							Operation: jsonpatch.OperationRemove,
							Path:      patchOp.Path,
							OldValue:  op.Value,
						},
					}
					break
				}
				src = jsonpatch.Patch{
					jsonpatch.Operation{
						Operation: jsonpatch.OperationReplace,
						Path:      patchOp.Path,
						Value:     patchOp.Value,
						OldValue:  op.Value,
					},
				}
			}

			if len(src) == 0 {
				src = jsonpatch.Patch{patchOp}
				break
			}

			var i int
			for i = range src {
				if src[i].Path.LessThan(patchOp.Path) {
					continue
				}

				i--
				break
			}

			i++
			src = slices.Insert(src, i, patchOp)

		case jsonpatch.OperationReplace:
			i, ok := findPatchIndex(src, patchOp.Path)
			if !ok {
				return nil, fmt.Errorf("path %q not found in original", patchOp.Path.String())
			}

			for j := i; j < len(src); j++ {
				if patchOp.Path.IsParentOf(src[j].Path) {
					src = slices.Delete(src, j, j+1)
					j--
				}
			}

			if f.jsonInJSONComparer != nil && src[i].UnmarshaledValue != nil && patchOp.UnmarshaledValue != nil {
				var diff jsonpatch.Patch
				err := func() error {
					jpatch, err := f.jsonInJSONComparer(src[i].UnmarshaledValue, patchOp.UnmarshaledValue)
					if err != nil {
						return err
					}

					originalPatchTestSeries, err := f.asPatchTestSeries(src[i].UnmarshaledValue, jsonpointer.NewPointer())
					if err != nil {
						return err
					}

					patch, err := f.patchFromAny(jpatch)
					if err != nil {
						return err
					}

					diff, err = f.compileDiffPatchSeries(originalPatchTestSeries, patch)
					if err != nil {
						return err
					}

					return nil
				}()
				// Only consider JSON in JSON if comparing does not return any error,
				// fall back to normal processing otherwise.
				if nil == err {
					for j := range diff {
						diff[j].Path = src[i].Path.Append(diff[j].Path)
					}

					src = slices.Replace(src, i, i+1, diff[0:]...)

					break
				}
			}

			patchOp.OldValue = src[i].Value
			src[i] = patchOp

		case jsonpatch.OperationRemove:
			i, ok := findPatchIndex(src, patchOp.Path)
			if !ok {
				return nil, fmt.Errorf("path %q not found in original", patchOp.Path.String())
			}

			patchOp.OldValue = src[i].Value
			if f.jsonInJSONComparer != nil && src[i].UnmarshaledValue != nil {
				patchOp.OldValue = src[i].UnmarshaledValue
			}
			src[i] = patchOp

			for j := opIndex + 1; j < len(patch); j++ {
				if patch[j].Path.HasSameAncestorsAs(patchOp.Path) && !patch[j].Path.LessThan(patchOp.Path) {
					patch[j].Path.IncrementIndex()
					continue
				}
				break
			}

			for j := i; j < len(src); j++ {
				if patchOp.Path.IsParentOf(src[j].Path) {
					src = slices.Delete(src, j, j+1)
					j--
				}
			}
		}
	}

	return src, nil
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

func (f Formatter) patchFromAny(value any) (jsonpatch.Patch, error) {
	var jsonbody []byte
	var err error

	switch t := value.(type) {
	case []byte:
		if len(t) == 0 {
			return jsonpatch.Patch{}, nil
		}
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

	if f.jsonInJSONComparer != nil {
		for i := range patch {
			if jsonValue, ok := asJSONInJSON(patch[i].Value); ok {
				patch[i].UnmarshaledValue = jsonValue
			}
		}
	}

	return patch, nil
}
