package jsondiffprinter

import (
	"sort"

	"github.com/wI2L/jsondiff"
)

const defaultPatchAllocationSize = 32

func ExhaustiveJSONPatchTests(value any) jsondiff.Patch {
	return exhaustiveJSONPatchTests(value, NewPointer())
}

func exhaustiveJSONPatchTests(value any, path Pointer) jsondiff.Patch {
	patches := make(jsondiff.Patch, 0, defaultPatchAllocationSize)

	switch t := value.(type) {
	case map[string]any:
		patches = append(patches, jsondiff.Operation{
			Type:  jsondiff.OperationTest,
			Path:  path.String(),
			Value: value,
		})

		for _, k := range keys(t) {
			patches = append(patches, exhaustiveJSONPatchTests(t[k], path.Append(k))...)
		}

	case []any:
		patches = append(patches, jsondiff.Operation{
			Type:  jsondiff.OperationTest,
			Path:  path.String(),
			Value: value,
		})

		for i, v := range t {
			patches = append(patches, exhaustiveJSONPatchTests(v, path.AppendIndex(i))...)
		}

	default:
		// TODO: be more strict about what we accept here, not everything is valid JSON.
		patches = append(patches, jsondiff.Operation{
			Type:  jsondiff.OperationTest,
			Path:  path.String(),
			Value: value,
		})
	}

	return patches
}

func keys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
