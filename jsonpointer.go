package jsondiffprinter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const defaultPointerAllocationSize = 32

type Pointer []string

// NewPointer creates a Pointer with a pre-allocated block of memory
// to avoid repeated slice expansions
func NewPointer() Pointer {
	return make([]string, 0, defaultPointerAllocationSize)
}

func NewPointerFromPath(path string) (Pointer, error) {
	if len(path) == 0 {
		return NewPointer(), nil
	}

	if path[0] != '/' {
		return nil, fmt.Errorf("non-empty references must begin with a '/' character")
	}
	path = path[1:]

	toks := strings.Split(path, separator)
	for i, t := range toks {
		toks[i] = unescapeToken(t)
	}
	return Pointer(toks), nil
}

func (p Pointer) Append(s string) Pointer {
	p = append(p, s)
	return p
}

func (p Pointer) AppendIndex(i int) Pointer {
	p = append(p, strconv.Itoa(i))
	return p
}

func (p Pointer) IsParentOf(child Pointer) bool {
	if len(p) != len(child)-1 {
		return false
	}
	for i := range p {
		if p[i] != child[i] {
			return false
		}
	}
	return true
}

func (p Pointer) String() string {
	if len(p) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, t := range p {
		sb.WriteString(separator)
		sb.WriteString(escapeToken(t))
	}
	return sb.String()
}

func (p Pointer) IsEmpty() bool {
	return len(p) == 0
}

func (p *Pointer) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*p = nil
		return nil
	}

	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	np, err := NewPointerFromPath(s)
	if err != nil {
		return err
	}

	*p = np
	return nil
}

func (p Pointer) MarshalJSON() ([]byte, error) {
	return []byte(`"` + p.String() + `"`), nil
}

const (
	separator        = "/"
	escapedSeparator = "~1"
	tilde            = "~"
	escapedTilde     = "~0"
)

func unescapeToken(tok string) string {
	tok = strings.ReplaceAll(tok, escapedSeparator, separator)
	return strings.ReplaceAll(tok, escapedTilde, tilde)
}

func escapeToken(tok string) string {
	tok = strings.ReplaceAll(tok, tilde, escapedTilde)
	return strings.ReplaceAll(tok, separator, escapedSeparator)
}
