package jsondiffprinter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
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

	jsonInJSONStartJSON      = "embeddedJSON("
	jsonInJSONEndJSON        = ")"
	jsonInJSONStartTerraform = "jsonencode("
	jsonInJSONEndTerraform   = `)`
)

// A Comparer compares two JSON documents and returns a JSON patch that
// transforms the first document into the second document.
type Comparer func(before, after any) ([]byte, error)

type Patch = jsonpatch.Patch

// A PatchSeriesPostProcessor processes the JSON patch series before the
// diff is printed. It can be used to modify the diff before it is printed.
type PatchSeriesPostProcessor func(diff Patch) Patch

// formatter formats the diff if the given JSON patch is applied to the given
// JSON document.
type formatter struct {
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
	jsonInJSONStart                      string
	jsonInJSONEnd                        string
	patchSeriesPostProcess               PatchSeriesPostProcessor
}

type valueType int

func (v valueType) leftBracket(f formatter) string {
	switch v {
	case valueTypePlain:
		return ""
	case valueTypeObject:
		return "{"
	case valueTypeJSONinJSONObject:
		return f.jsonInJSONStart
	case valueTypeArray:
		return "["
	case valueTypeJSONinJSONArray:
		return f.jsonInJSONStart
	default:
		panic("undefined value type")
	}
}

func (v valueType) rightBracket(f formatter) string {
	switch v {
	case valueTypePlain:
		return ""
	case valueTypeObject:
		return "}"
	case valueTypeJSONinJSONObject:
		return f.jsonInJSONEnd
	case valueTypeArray:
		return "]"
	case valueTypeJSONinJSONArray:
		return f.jsonInJSONEnd
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

type notePosition int

const (
	notePositionKey notePosition = iota
	notePositionValue
	notePositionEnd
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
//
// Format accepts Options to configure the format and the destination.
func Format(original any, jsonpatch any, options ...Option) error {
	f := formatter{
		w: os.Stdout,
		c: colorize{
			disable: true,
		},

		indentation:       "  ",
		commas:            true,
		keyValueSeparator: keyValueSeparatorJSON,
		keyQuote:          keyQuoteJSON,
		jsonInJSONStart:   jsonInJSONStartJSON,
		jsonInJSONEnd:     jsonInJSONEndJSON,
	}

	for _, option := range options {
		option(&f)
	}

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

func (f formatter) printPatch(patch jsonpatch.Patch, parentPath jsonpointer.Pointer, isArray bool) (int, bool) {
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

func (f formatter) printOp(cfg printOpConfig) {
	if cfg.op.Operation == jsonpatch.OperationReplace && !f.singleLineReplace {
		if cfg.valType != valueTypeJSONinJSONObject && cfg.valType != valueTypeJSONinJSONArray {
			op := cfg.op.Clone()
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
			fmt.Fprintf(f.w, "%s\n", cfg.valueOldComma)
		}

		op := cfg.op.Clone()
		op.Operation = jsonpatch.OperationAdd
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
	if len(cfg.valType.leftBracket(f)) > 0 {
		leftBracket = cfg.valType.leftBracket(f) + "\n"
	}
	switch cfg.valType.notePos() {
	case notePositionKey:
		leftBracket = cfg.valType.leftBracket(f) + cfg.op.Metadata["note"] + "\n"
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
	if cfg.valType.rightBracket(f) != "" {
		fmt.Fprintf(f.w, "%s  %s%s%s", cfg.preDiffMarkerIndent, cfg.indent, cfg.valType.rightBracket(f), endNote)
	}
}

func (f formatter) opTypeIndicator(opType jsonpatch.OperationType) string {
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
func (f formatter) printCommaOrNot(i int, patch jsonpatch.Patch, op jsonpatch.Operation) string {
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

func (f formatter) formatIndent(v any, prefix string, operation string) string {
	switch vt := v.(type) {
	case jsonInJSONObject:
		sb := strings.Builder{}
		sb.WriteString(f.jsonInJSONStart + "\n")
		sb.WriteString(f.prefix + prefix + f.indentation + "  ")
		sb.WriteString(f.formatIndent(map[string]any(vt), prefix+f.indentation, operation))
		sb.WriteString("\n" + f.prefix + prefix + "  " + f.jsonInJSONEnd)
		return sb.String()
	case jsonInJSONArray:
		sb := strings.Builder{}
		sb.WriteString(f.jsonInJSONStart + "\n")
		sb.WriteString(f.prefix + prefix + f.indentation + "  ")
		sb.WriteString(f.formatIndent([]any(vt), prefix+f.indentation, operation))
		sb.WriteString("\n" + f.prefix + prefix + "  " + f.jsonInJSONEnd)
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

func keys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
