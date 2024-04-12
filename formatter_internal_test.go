package jsondiffprinter

import (
	"testing"

	"github.com/breml/jsondiffprinter/internal/jsonpatch"
	"github.com/breml/jsondiffprinter/internal/jsonpointer"
	"github.com/breml/jsondiffprinter/internal/require"
)

func Test_compileDiffPatchSeries(t *testing.T) {
	tt := []struct {
		name string

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

			assertErr: require.Error,
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
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := compileDiffPatchSeries(nil, tc.patch)
			tc.assertErr(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
