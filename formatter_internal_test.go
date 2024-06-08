package jsondiffprinter

import (
	"testing"

	"github.com/breml/jsondiffprinter/internal/jsonpatch"
	"github.com/breml/jsondiffprinter/internal/jsonpointer"
	"github.com/breml/jsondiffprinter/internal/require"
)

func Test_compileDiffPatchSeries(t *testing.T) {
	tests := []struct {
		name string

		src   jsonpatch.Patch
		patch jsonpatch.Patch

		assertErr require.ErrorAssertionFunc
		want      jsonpatch.Patch
	}{
		{
			name: "empty source, empty patch",

			assertErr: require.NoError,
			want:      jsonpatch.Patch{},
		},
		{
			name: "empty source, add patch",

			patch: jsonpatch.Patch{
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointer(), Value: "value"},
			},

			assertErr: require.NoError,
			want: jsonpatch.Patch{
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointer(), Value: "value"},
			},
		},
		{
			name: "empty source, test patch",

			patch: jsonpatch.Patch{
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointer(), Value: "value"},
			},

			assertErr: require.NoError,
			want:      jsonpatch.Patch{},
		},
		{
			name: "empty source, replace patch",

			patch: jsonpatch.Patch{
				{Operation: jsonpatch.OperationReplace, Path: jsonpointer.NewPointer(), Value: "value"},
			},

			assertErr: require.Error,
		},
		{
			name: "empty source, remove patch",

			patch: jsonpatch.Patch{
				{Operation: jsonpatch.OperationRemove, Path: jsonpointer.NewPointer(), Value: "value"},
			},

			assertErr: require.Error,
		},

		{
			name: "array append with index",

			src: jsonpatch.Patch{
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointer(), Value: map[string]any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array"), Value: []any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/0"), Value: 0},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 1},
			},
			patch: jsonpatch.Patch{
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/2"), Value: 2},
			},

			assertErr: require.NoError,
			want: jsonpatch.Patch{
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointer(), Value: map[string]any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array"), Value: []any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/0"), Value: 0},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 1},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/2"), Value: 2},
			},
		},
		{
			name: "array insert",

			src: jsonpatch.Patch{
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointer(), Value: map[string]any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array"), Value: []any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/0"), Value: 0},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 2},
			},
			patch: jsonpatch.Patch{
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 1},
			},

			assertErr: require.NoError,
			want: jsonpatch.Patch{
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointer(), Value: map[string]any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array"), Value: []any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/0"), Value: 0},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 1},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 2},
			},
		},
		{
			name: "array insert without LCS",

			src: jsonpatch.Patch{
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointer(), Value: map[string]any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array"), Value: []any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/0"), Value: 0},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 2},
			},
			patch: jsonpatch.Patch{
				{Operation: jsonpatch.OperationReplace, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 1},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/-"), Value: 2},
			},

			assertErr: require.NoError,
			want: jsonpatch.Patch{
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointer(), Value: map[string]any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array"), Value: []any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/0"), Value: 0},
				{Operation: jsonpatch.OperationReplace, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 1, OldValue: 2},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/-"), Value: 2},
			},
		},
		{
			name: "array multi change",

			src: jsonpatch.Patch{
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointer(), Value: map[string]any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array"), Value: []any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/0"), Value: 5},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 6},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/2"), Value: 7},
			},
			patch: jsonpatch.Patch{
				{Operation: jsonpatch.OperationRemove, Path: jsonpointer.NewPointerFromPath("/array/1")},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/2"), Value: 8},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/3"), Value: 9},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/4"), Value: 10},
			},

			assertErr: require.NoError,
			want: jsonpatch.Patch{
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointer(), Value: map[string]any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array"), Value: []any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/0"), Value: 5},
				{Operation: jsonpatch.OperationRemove, Path: jsonpointer.NewPointerFromPath("/array/1"), OldValue: 6},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/2"), Value: 7},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/3"), Value: 8},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/4"), Value: 9},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/5"), Value: 10},
			},
		},
		{
			name: "array multi change without LCS",

			src: jsonpatch.Patch{
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointer(), Value: map[string]any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array"), Value: []any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/0"), Value: 5},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 6},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/2"), Value: 7},
			},
			patch: jsonpatch.Patch{
				{Operation: jsonpatch.OperationReplace, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 7},
				{Operation: jsonpatch.OperationReplace, Path: jsonpointer.NewPointerFromPath("/array/2"), Value: 8},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/-"), Value: 9},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/-"), Value: 10},
			},

			assertErr: require.NoError,
			want: jsonpatch.Patch{
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointer(), Value: map[string]any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array"), Value: []any{}},
				{Operation: jsonpatch.OperationTest, Path: jsonpointer.NewPointerFromPath("/array/0"), Value: 5},
				{Operation: jsonpatch.OperationReplace, Path: jsonpointer.NewPointerFromPath("/array/1"), Value: 7, OldValue: 6},
				{Operation: jsonpatch.OperationReplace, Path: jsonpointer.NewPointerFromPath("/array/2"), Value: 8, OldValue: 7},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/-"), Value: 9},
				{Operation: jsonpatch.OperationAdd, Path: jsonpointer.NewPointerFromPath("/array/-"), Value: 10},
			},
		},
	}

	f := Formatter{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.compileDiffPatchSeries(tc.src, tc.patch)
			tc.assertErr(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
