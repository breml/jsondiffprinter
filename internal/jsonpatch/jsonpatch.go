package jsonpatch

import (
	"encoding/json"
	"fmt"

	"github.com/breml/jsondiffprinter/internal/jsonpointer"
)

type OperationType string

// JSON Patch operation types.
// These are defined in RFC 6902 section 4.
// https://datatracker.ietf.org/doc/html/rfc6902#section-4
const (
	OperationAdd     OperationType = "add"
	OperationReplace OperationType = "replace"
	OperationRemove  OperationType = "remove"
	// FIXME: Operation move and copy are not supported by this package.
	// OperationMove OperationType = "move"
	// OperationCopy OperationType = "copy"
	OperationTest OperationType = "test"
)

func (o *OperationType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"add"`:
		*o = OperationAdd
	case `"replace"`:
		*o = OperationReplace
	case `"remove"`:
		*o = OperationRemove
	case `"test"`:
		*o = OperationTest
	default:
		return fmt.Errorf("unknown operation type: %s", string(data))
	}
	return nil
}

// Patch represents a series of JSON Patch operations.
type Patch []Operation

func (p Patch) GoString() string {
	jsonBody, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Sprintf("<invalid: failed to json marshal patch: %v\n", err)
	}
	return string(jsonBody) + "\n"
}

// Operation represents a single JSON Patch (RFC6902) operation.
type Operation struct {
	Value     any           `json:"value,omitempty"`
	OldValue  any           `json:"oldValue"`
	Operation OperationType `json:"op"`
	// NOTE: From is not used by this package as of now, since we do not support move and copy operations.
	// From string `json:"from,omitempty"`
	Path jsonpointer.Pointer `json:"path"`

	UnmarshaledValue any               `json:"unmarshaledValue"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

func (o Operation) Clone() Operation {
	clone := o
	clone.Metadata = make(map[string]string, len(o.Metadata))
	for k, v := range o.Metadata {
		clone.Metadata[k] = v
	}

	return clone
}

func (o Operation) GoString() string {
	jsonBody, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return fmt.Sprintf("<invalid: failed to json marshal operation: %v\n", err)
	}
	return string(jsonBody) + "\n"
}
