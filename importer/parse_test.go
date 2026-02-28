package importer

import "testing"

func TestParseMinutes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "empty", input: "", want: 0},
		{name: "integer minutes", input: "8", want: 8},
		{name: "decimal dot", input: "7.5", want: 8},
		{name: "decimal comma", input: "7,4", want: 7},
		{name: "negative", input: "-1", wantErr: true},
		{name: "invalid", input: "abc", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseMinutes(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("unexpected minutes for %q: want %d, got %d", tc.input, tc.want, got)
			}
		})
	}
}
