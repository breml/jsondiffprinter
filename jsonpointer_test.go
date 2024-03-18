package jsondiffprinter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/breml/jsondiffprinter"
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
		{
			name: "error - no leading slash",

			json: `{"pointer": "foo"}`,

			assertErr: require.Error,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			jsonPointer := struct {
				Pointer jsondiffprinter.Pointer `json:"pointer"`
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

		pointer jsondiffprinter.Pointer

		want string
	}{
		{
			name: "success - no pointer",

			pointer: jsondiffprinter.Pointer(nil),

			want: `{"pointer":""}`,
		},
		{
			name: "success - empty string",

			pointer: jsondiffprinter.Pointer([]string{}),

			want: `{"pointer":""}`,
		},
		{
			name: "success - single slash",

			pointer: jsondiffprinter.Pointer([]string{""}),

			want: `{"pointer":"/"}`,
		},
		{
			name: "success - single slash",

			pointer: jsondiffprinter.Pointer([]string{"foo", "bar", "baz", "1"}),

			want: `{"pointer":"/foo/bar/baz/1"}`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			jsonPointer := struct {
				Pointer jsondiffprinter.Pointer `json:"pointer"`
			}{
				Pointer: tc.pointer,
			}

			b, err := json.Marshal(jsonPointer)
			require.NoError(t, err)

			require.JSONEq(t, tc.want, string(b))
		})
	}
}

func TestPointerAppend(t *testing.T) {
	tt := []struct {
		name string

		pointer jsondiffprinter.Pointer
		token   string

		want jsondiffprinter.Pointer
	}{
		{
			name: "success - empty pointer",

			pointer: jsondiffprinter.Pointer(nil),
			token:   "foo",

			want: jsondiffprinter.Pointer([]string{"foo"}),
		},
		{
			name: "success - non-empty pointer",

			pointer: jsondiffprinter.Pointer([]string{"foo", "bar"}),
			token:   "baz",

			want: jsondiffprinter.Pointer([]string{"foo", "bar", "baz"}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.pointer.Append(tc.token))
		})
	}
}

func TestPointerAppendIndex(t *testing.T) {
	tt := []struct {
		name string

		pointer jsondiffprinter.Pointer
		index   int

		want jsondiffprinter.Pointer
	}{
		{
			name: "success - empty pointer",

			pointer: jsondiffprinter.Pointer(nil),
			index:   1,

			want: jsondiffprinter.Pointer([]string{"1"}),
		},
		{
			name: "success - non-empty pointer",

			pointer: jsondiffprinter.Pointer([]string{"foo", "bar"}),
			index:   2,

			want: jsondiffprinter.Pointer([]string{"foo", "bar", "2"}),
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

		parent jsondiffprinter.Pointer
		child  jsondiffprinter.Pointer

		want bool
	}{
		{
			name: "success - empty pointers",

			parent: jsondiffprinter.Pointer(nil),
			child:  jsondiffprinter.Pointer(nil),

			want: false,
		},
		{
			name: "success - empty parent",

			parent: jsondiffprinter.Pointer(nil),
			child:  jsondiffprinter.Pointer([]string{"foo"}),

			want: true,
		},
		{
			name: "success - empty child",

			parent: jsondiffprinter.Pointer([]string{"foo"}),
			child:  jsondiffprinter.Pointer(nil),

			want: false,
		},
		{
			name: "success - equal pointers",

			parent: jsondiffprinter.Pointer([]string{"foo", "bar", "baz"}),
			child:  jsondiffprinter.Pointer([]string{"foo", "bar", "baz"}),

			want: false,
		},
		{
			name: "success - parent is parent of child",

			parent: jsondiffprinter.Pointer([]string{"foo", "bar"}),
			child:  jsondiffprinter.Pointer([]string{"foo", "bar", "baz"}),

			want: true,
		},
		{
			name: "success - parent is not parent of child",

			parent: jsondiffprinter.Pointer([]string{"foo", "baz"}),
			child:  jsondiffprinter.Pointer([]string{"foo", "bar", "baz"}),

			want: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.parent.IsParentOf(tc.child))
		})
	}
}
