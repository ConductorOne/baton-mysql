package client

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoutineTypesFromRows(t *testing.T) {
	tests := []struct {
		name    string
		rows    []string
		want    string
		wantErr string
	}{
		{
			name:    "empty rows -> not found",
			rows:    []string{},
			wantErr: "not found",
		},
		{
			name: "single procedure",
			rows: []string{"PROCEDURE"},
			want: "PROCEDURE",
		},
		{
			name: "single function",
			rows: []string{"FUNCTION"},
			want: "FUNCTION",
		},
		{
			name: "duplicate procedure rows",
			rows: []string{"PROCEDURE", "PROCEDURE"},
			want: "PROCEDURE",
		},
		{
			name:    "procedure and function collision -> ambiguous",
			rows:    []string{"PROCEDURE", "FUNCTION"},
			wantErr: "ambiguous routine name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := routineTypesFromRows(tt.rows, "db1", "p1")
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRoutineTypesFromRowsNotFoundWrapsNoRows(t *testing.T) {
	_, err := routineTypesFromRows(nil, "db1", "p1")
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "baton-mysql: "), "error must carry the baton-mysql prefix, got: %s", err.Error())
}
