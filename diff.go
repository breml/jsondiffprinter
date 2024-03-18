package jsondiffprinter

import (
	"sort"
	"strings"

	"github.com/wI2L/jsondiff"
)

func ApplyPatch(src jsondiff.Patch, patch jsondiff.Patch) jsondiff.Patch {
	res := make(jsondiff.Patch, 0, len(src))
	for _, op := range src {
		patchop, ok := findPatch(patch, op.Path)
		if !ok {
			res = append(res, op)
			continue
		}

		if patchop.Type == jsondiff.OperationAdd && patchop.Path == "" {
			// If incomparable values are located at the root
			// of the document, an add operation to replace
			// the entire content of the document is provided.
			// https://tools.ietf.org/html/rfc6902#section-4.1
			//
			// We replace this operation with a replace operation
			// to make the change of the value explicit.
			return jsondiff.Patch{
				jsondiff.Operation{
					Type:     jsondiff.OperationReplace,
					Path:     patchop.Path,
					Value:    patchop.Value,
					OldValue: op.Value,
				},
			}
		}

		res = append(res, patchop)
	}

	for _, op := range patch {
		if op.Type == jsondiff.OperationAdd {
			res = append(res, op)
		}
	}

	sort.Slice(res, func(i, j int) bool {
		if (strings.HasSuffix(res[i].Path, "/-") || strings.HasSuffix(res[j].Path, "/-")) && res[i].Path[:len(res[i].Path)-2] == res[j].Path[:len(res[j].Path)-2] {
			if strings.HasSuffix(res[i].Path, "/-") {
				return false
			}
			return true
		}
		return res[i].Path < res[j].Path
	})

	return res
}

func findPatch(patch jsondiff.Patch, path string) (jsondiff.Operation, bool) {
	for _, op := range patch {
		if op.Path == path {
			return op, true
		}
	}
	return jsondiff.Operation{}, false
}
