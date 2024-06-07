package jsonpointer

import (
	"encoding/json"
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

func NewPointerFromPath(path string) Pointer {
	if len(path) == 0 {
		return NewPointer()
	}

	// According to the RFC, non-empty references must begin with a '/' character.
	// For simplification purposes, we just assume the '/' character is present.
	if path[0] == '/' {
		path = path[1:]
	}

	toks := strings.Split(path, separator)
	for i, t := range toks {
		toks[i] = unescapeToken(t)
	}
	return Pointer(toks)
}

func (p Pointer) Append(s string) Pointer {
	pp := make(Pointer, 0, len(p)+1)
	pp = append(pp, p...)
	pp = append(pp, s)
	return pp
}

func (p Pointer) AppendIndex(i int) Pointer {
	pp := make(Pointer, 0, len(p)+1)
	pp = append(pp, p...)
	pp = append(pp, strconv.Itoa(i))
	return pp
}

func (p *Pointer) IncrementIndex() {
	if len(*p) == 0 {
		return
	}
	i, err := strconv.ParseInt((*p)[len(*p)-1], 10, 64)
	if err != nil {
		return
	}
	(*p)[len(*p)-1] = strconv.Itoa(int(i + 1))
}

func (p *Pointer) DecrementIndex() {
	if len(*p) == 0 {
		return
	}
	i, err := strconv.ParseInt((*p)[len(*p)-1], 10, 64)
	if err != nil {
		return
	}
	(*p)[len(*p)-1] = strconv.Itoa(int(i - 1))
}

func (p Pointer) LessThan(alt Pointer) (b bool) {
	if p.HasSameAncestorsAs(alt) && (p[len(p)-1] == "-" || alt[len(alt)-1] == "-") {
		if p[len(p)-1] == "-" && alt[len(alt)-1] == "-" {
			return true
		}
		return p[len(p)-1] != "-"
	}
	for i := 0; i < min(len(p), len(alt)); i++ {
		if p[i] != alt[i] {
			pi, perr := strconv.Atoi(p[i])
			alti, alterr := strconv.Atoi(alt[i])
			if perr == nil && alterr == nil {
				return pi < alti
			}
			return p[i] < alt[i]
		}
	}
	return len(p) < len(alt)
}

func (p Pointer) Equals(alt Pointer) bool {
	return equal(p, alt)
}

func (p Pointer) IsParentOf(child Pointer) bool {
	if len(child) < 1 {
		return false
	}
	return equal(p, child[:len(child)-1])
}

func (p Pointer) HasSameAncestorsAs(alt Pointer) bool {
	if len(p) < 1 || len(alt) < 1 {
		return false
	}
	return equal(p[:len(p)-1], alt[:len(alt)-1])
}

func equal(a, b Pointer) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
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

	np := NewPointerFromPath(s)

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
