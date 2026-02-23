package releasepolicy

import "testing"

func TestIsStableSemverTagRef(t *testing.T) {
	tests := []struct {
		ref  string
		want bool
	}{
		{ref: "refs/tags/v1.2.3", want: true},
		{ref: "refs/tags/v0.0.1", want: true},
		{ref: "refs/tags/v1.2.3-rc1", want: false},
		{ref: "refs/tags/v1.2", want: false},
		{ref: "refs/heads/main", want: false},
	}

	for _, tt := range tests {
		if got := IsStableSemverTagRef(tt.ref); got != tt.want {
			t.Fatalf("unexpected stable result for %q: got %v want %v", tt.ref, got, tt.want)
		}
	}
}
