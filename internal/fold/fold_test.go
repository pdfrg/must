package fold

import "testing"

func TestString(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Beyoncé", "beyonce"},
		{"Sigur Rós", "sigur ros"},
		{"Björk", "bjork"},
		{"Café Tacvba", "cafe tacvba"},
		{"Hoppípolla", "hoppipolla"},
		{"Jóga", "joga"},
		{"BEYONCÉ", "beyonce"},
		{"plain ascii", "plain ascii"},
		{"", ""},
		// Non-Latin scripts pass through (lower-cased only).
		{"日本語", "日本語"},
	}
	for _, tc := range cases {
		if got := String(tc.in); got != tc.want {
			t.Errorf("String(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStringIdempotent(t *testing.T) {
	inputs := []string{"Beyoncé", "Sigur Rós", "plain", ""}
	for _, in := range inputs {
		once, twice := String(in), String(String(in))
		if once != twice {
			t.Errorf("String not idempotent for %q: %q vs %q", in, once, twice)
		}
	}
}

func TestEqual(t *testing.T) {
	if !Equal("Beyoncé", "beyonce") {
		t.Error("Equal(Beyoncé, beyonce) = false, want true")
	}
	if !Equal("Beyoncé", "BEYONCÉ") {
		t.Error("Equal(Beyoncé, BEYONCÉ) = false, want true")
	}
	if Equal("Beyoncé", "beyonc") {
		t.Error("Equal(Beyoncé, beyonc) = true, want false")
	}
	if Equal("Halo", "beyonce") {
		t.Error("Equal(Halo, beyonce) = true, want false")
	}
}

func TestContains(t *testing.T) {
	if !Contains("Beyoncé", "beyonce") {
		t.Error("Contains(Beyoncé, beyonce) = false, want true")
	}
	if !Contains("Sigur Rós", "gur r") {
		t.Error("Contains(Sigur Rós, gur r) = false, want true")
	}
	if Contains("Halo", "beyonce") {
		t.Error("Contains(Halo, beyonce) = true, want false")
	}
}
