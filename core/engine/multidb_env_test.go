package engine

import (
	"testing"
)

func TestForceLiveDiscovery(t *testing.T) {
	for _, tc := range []struct {
		val  string
		want bool
	}{
		{"", false},
		{"0", false},
		{"no", false},
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{" 1 ", true},
	} {
		t.Setenv("GRAPHJIN_FORCE_DISCOVERY", tc.val)
		if got := forceLiveDiscovery(); got != tc.want {
			t.Errorf("env %q: got %v, want %v", tc.val, got, tc.want)
		}
	}
}
