package jsondiffprinter

import (
	"github.com/breml/jsondiffprinter/internal/jsonpatch"
	"github.com/breml/jsondiffprinter/internal/jsonpointer"
)

func FindPatchIndex(patch jsonpatch.Patch, path jsonpointer.Pointer) (int, bool) {
	return findPatchIndex(patch, path)
}
