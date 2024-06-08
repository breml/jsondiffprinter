package jsonpointer_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/breml/jsondiffprinter/internal/jsonpointer"
	"github.com/breml/jsondiffprinter/internal/require"
)

func TestPointerFromJSON(t *testing.T) {
	tt := []struct {
		name string

		json string

		assertErr require.ErrorAssertionFunc
		want      string
	}{
		{
			name: "success - no pointer",

			json: `{}`,

			assertErr: require.NoError,
			want:      "",
		},
		{
			name: "success - null pointer",

			json: `{"pointer": null}`,

			assertErr: require.NoError,
			want:      "",
		},
		{
			name: "success - empty string",

			json: `{"pointer": ""}`,

			assertErr: require.NoError,
			want:      "",
		},
		{
			name: "success - single slash",

			json: `{"pointer": "/"}`,

			assertErr: require.NoError,
			want:      "/",
		},
		{
			name: "success - single slash",

			json: `{"pointer": "/foo/bar/baz/1"}`,

			assertErr: require.NoError,
			want:      "/foo/bar/baz/1",
		},
		{
			name: "error - invalid type for pointer",

			json: `{"pointer": false}`,

			assertErr: require.Error,
		},
		// This is not according to the RFC, but we allow it for convenience.
		{
			name: "no leading slash",

			json: `{"pointer": "foo"}`,

			assertErr: require.NoError,
			want:      "/foo",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			jsonPointer := struct {
				Pointer jsonpointer.Pointer `json:"pointer"`
			}{}

			err := json.Unmarshal([]byte(tc.json), &jsonPointer)
			tc.assertErr(t, err)

			require.Equal(t, jsonPointer.Pointer.String(), tc.want)
		})
	}
}

func TestPointerToJSON(t *testing.T) {
	tt := []struct {
		name string

		pointer jsonpointer.Pointer

		want string
	}{
		{
			name: "success - no pointer",

			pointer: jsonpointer.Pointer(nil),

			want: `{"pointer":""}`,
		},
		{
			name: "success - empty string",

			pointer: jsonpointer.Pointer([]string{}),

			want: `{"pointer":""}`,
		},
		{
			name: "success - single slash",

			pointer: jsonpointer.Pointer([]string{""}),

			want: `{"pointer":"/"}`,
		},
		{
			name: "success - single slash",

			pointer: jsonpointer.Pointer([]string{"foo", "bar", "baz", "1"}),

			want: `{"pointer":"/foo/bar/baz/1"}`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			jsonPointer := struct {
				Pointer jsonpointer.Pointer `json:"pointer"`
			}{
				Pointer: tc.pointer,
			}

			b, err := json.Marshal(jsonPointer)
			require.NoError(t, err)

			require.Equal(t, tc.want, string(b))
		})
	}
}

func TestPointerAppend(t *testing.T) {
	tt := []struct {
		name string

		pointer jsonpointer.Pointer
		token   string

		want jsonpointer.Pointer
	}{
		{
			name: "success - empty pointer",

			pointer: jsonpointer.Pointer(nil),
			token:   "foo",

			want: jsonpointer.Pointer([]string{"foo"}),
		},
		{
			name: "success - non-empty pointer",

			pointer: jsonpointer.Pointer([]string{"foo", "bar"}),
			token:   "baz",

			want: jsonpointer.Pointer([]string{"foo", "bar", "baz"}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.pointer.AppendKey(tc.token))
		})
	}
}

func TestPointerAppendIndex(t *testing.T) {
	tt := []struct {
		name string

		pointer jsonpointer.Pointer
		index   int

		want jsonpointer.Pointer
	}{
		{
			name: "success - empty pointer",

			pointer: jsonpointer.Pointer(nil),
			index:   1,

			want: jsonpointer.Pointer([]string{"1"}),
		},
		{
			name: "success - non-empty pointer",

			pointer: jsonpointer.Pointer([]string{"foo", "bar"}),
			index:   2,

			want: jsonpointer.Pointer([]string{"foo", "bar", "2"}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.pointer.AppendIndex(tc.index))
		})
	}
}

func TestPointerIsParentOf(t *testing.T) {
	tt := []struct {
		name string

		parent jsonpointer.Pointer
		child  jsonpointer.Pointer

		want bool
	}{
		{
			name: "success - empty pointers",

			parent: jsonpointer.Pointer(nil),
			child:  jsonpointer.Pointer(nil),

			want: false,
		},
		{
			name: "success - empty parent",

			parent: jsonpointer.Pointer(nil),
			child:  jsonpointer.Pointer([]string{"foo"}),

			want: true,
		},
		{
			name: "success - empty child",

			parent: jsonpointer.Pointer([]string{"foo"}),
			child:  jsonpointer.Pointer(nil),

			want: false,
		},
		{
			name: "success - equal pointers",

			parent: jsonpointer.Pointer([]string{"foo", "bar", "baz"}),
			child:  jsonpointer.Pointer([]string{"foo", "bar", "baz"}),

			want: false,
		},
		{
			name: "success - parent is parent of child",

			parent: jsonpointer.Pointer([]string{"foo", "bar"}),
			child:  jsonpointer.Pointer([]string{"foo", "bar", "baz"}),

			want: true,
		},
		{
			name: "success - parent is not parent of child",

			parent: jsonpointer.Pointer([]string{"foo", "baz"}),
			child:  jsonpointer.Pointer([]string{"foo", "bar", "baz"}),

			want: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.parent.IsParentOf(tc.child))
		})
	}
}

func TestPointerIsAncestorOf(t *testing.T) {
	tt := []struct {
		name string

		ancestor  jsonpointer.Pointer
		successor jsonpointer.Pointer

		want bool
	}{
		{
			name: "success - empty pointers",

			ancestor:  jsonpointer.Pointer(nil),
			successor: jsonpointer.Pointer(nil),

			want: false,
		},
		{
			name: "success - empty ancestor",

			ancestor:  jsonpointer.Pointer(nil),
			successor: jsonpointer.Pointer([]string{"foo"}),

			want: true,
		},
		{
			name: "success - empty successor",

			ancestor:  jsonpointer.Pointer([]string{"foo"}),
			successor: jsonpointer.Pointer(nil),

			want: false,
		},
		{
			name: "success - equal pointers",

			ancestor:  jsonpointer.Pointer([]string{"foo", "bar", "baz"}),
			successor: jsonpointer.Pointer([]string{"foo", "bar", "baz"}),

			want: false,
		},
		{
			name: "success - ancestor is parent of successor",

			ancestor:  jsonpointer.Pointer([]string{"foo", "bar"}),
			successor: jsonpointer.Pointer([]string{"foo", "bar", "baz"}),

			want: true,
		},
		{
			name: "success - ancestor is not parent of successor",

			ancestor:  jsonpointer.Pointer([]string{"foo", "baz"}),
			successor: jsonpointer.Pointer([]string{"foo", "bar", "baz"}),

			want: false,
		},
		{
			name: "success - ancestor is grand-parent of successor",

			ancestor:  jsonpointer.Pointer([]string{"foo"}),
			successor: jsonpointer.Pointer([]string{"foo", "bar", "baz"}),

			want: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.ancestor.IsAncestorOf(tc.successor))
		})
	}
}

func TestPointerLessThan(t *testing.T) {
	tt := []struct {
		name string
		want bool
	}{
		{
			name: " < ", // root < root
			want: false,
		},
		{
			name: " < /child", // root < /child
			want: true,
		},
		{
			name: "/a < /b",
			want: true,
		},
		{
			name: "/1 < /2",
			want: true,
		},
		{
			name: "/1 < /-",
			want: true,
		},
		{
			name: "/- < /-",
			want: true,
		},
		{
			name: "/a < /b/-",
			want: true,
		},
		{
			name: "/a/5 < /a/10",
			want: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			parts := strings.SplitN(tc.name, "<", 2)
			a := jsonpointer.NewPointerFromPath(strings.TrimSpace(parts[0]))
			b := jsonpointer.NewPointerFromPath(strings.TrimSpace(parts[1]))

			require.Equal(t, tc.want, a.LessThan(b))
		})
	}
}
