package require

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"text/tabwriter"
)

type ErrorAssertionFunc func(t *testing.T, err error)

func NoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Error(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}
}

func Equal(t *testing.T, want, got any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("unexpected value: want:\n%v, got:\n%v\n%s", want, got, withTabwriter(fmt.Sprintf("%#v", want), fmt.Sprintf("%#v", got)))
	}
}

func EqualStringWithTabwriter(t *testing.T, want, got string) {
	t.Helper()
	if want == got {
		return
	}

	t.Fatal(withTabwriter(want, got))
}

func withTabwriter(want, got string) string {
	buf := bytes.NewBufferString("\n")
	w := tabwriter.NewWriter(buf, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "want:\tgot:\n=====\t====\n\t")

	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")

	minLens := min(len(wantLines), len(gotLines))
	for i := 0; i < minLens; i++ {
		fmt.Fprintf(w, "%s\t%s\n", wantLines[i], gotLines[i])
	}
	for i := minLens; i < len(wantLines); i++ {
		fmt.Fprintf(w, "%s\n", wantLines[i])
	}
	for i := minLens; i < len(gotLines); i++ {
		fmt.Fprintf(w, "\t%s\n", gotLines[i])
	}
	w.Flush()

	return buf.String()
}
