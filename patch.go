package jsondiffprinter

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/breml/jsondiffprinter/internal/jsonpatch"
	"github.com/breml/jsondiffprinter/internal/jsonpointer"
)

const defaultPatchAllocationSize = 32

func (f formatter) asPatchTestSeries(inValue any, path jsonpointer.Pointer) (jsonpatch.Patch, error) {
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

func (f formatter) patchFromAny(value any) (jsonpatch.Patch, error) {
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

func (f formatter) compileDiffPatchSeries(src jsonpatch.Patch, patch jsonpatch.Patch) (jsonpatch.Patch, error) {
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
