package cmd

import "testing"

func TestResolveReconcileMode(t *testing.T) {
	tests := []struct {
		name          string
		mode          string
		configDefault bool
		want          bool
		wantErr       bool
	}{
		{name: "auto true", mode: "auto", configDefault: true, want: true},
		{name: "auto false", mode: "auto", configDefault: false, want: false},
		{name: "empty uses config", mode: "", configDefault: true, want: true},
		{name: "on", mode: "on", configDefault: false, want: true},
		{name: "off", mode: "off", configDefault: true, want: false},
		{name: "yes alias", mode: "yes", configDefault: false, want: true},
		{name: "no alias", mode: "no", configDefault: true, want: false},
		{name: "invalid", mode: "maybe", configDefault: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveReconcileMode(tt.mode, tt.configDefault)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected value: expected %v, got %v", tt.want, got)
			}
		})
	}
}
