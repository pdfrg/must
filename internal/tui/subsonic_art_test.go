package tui

import "testing"

func TestIsNoStore(t *testing.T) {
	tests := []struct {
		name         string
		cacheControl string
		want         bool
	}{
		{"placeholder", "no-store", true},
		{"uppercase", "NO-STORE", true},
		{"mixed list", "public, no-store", true},
		{"revalidate", "public, no-cache", false},
		{"immutable", "public, max-age=31536000, immutable", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNoStore(tt.cacheControl); got != tt.want {
				t.Errorf("isNoStore(%q) = %v, want %v", tt.cacheControl, got, tt.want)
			}
		})
	}
}
