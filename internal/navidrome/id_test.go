package navidrome

import "testing"

// Golden vectors are copied verbatim from Navidrome's own tests:
// model/id/id_test.go and db/migrations/id_canonical_test.go (v0.64.0).
func TestCanonical(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"hash-family id fits 128 bits", "5cLJPkLA5DK2BADhoeotPk", "5cLJPkLA5DK2BADhoeotPk"},
		{"overflowing random id remapped via md5", "zzzzzzzzzzzzzzzzzzzzzz", "3LyqmwQBm5IRqlVjNYASwb"},
		{"legacy 32-hex re-encoded", "e3b7fc2ae9447bbec37a13bf916e3cf6", "6VHl3uR4kss6sUPKA8Cwnk"},
		{"playlist uuid re-encoded", "f47ac10b-58cc-4372-a567-0e02b2c3d479", "7rke2SAWaicSeSYzkhww6R"},
		{"empty passes through", "", ""},
		{"share id passes through", "aB3xY9kQz1", "aB3xY9kQz1"},
		{"truncated id passes through", "0123456789abcdef", "0123456789abcdef"},
		{"non-base62 22-char passes through", "!!!!!!!!!!!!!!!!!!!!!!", "!!!!!!!!!!!!!!!!!!!!!!"},
		{"non-hex 32-char passes through", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
		{"36-char without dashes passes through", "000000000000000000000000000000000000", "000000000000000000000000000000000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Canonical(tt.in); got != tt.want {
				t.Errorf("Canonical(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanonicalIdempotent(t *testing.T) {
	for _, s := range []string{
		"5cLJPkLA5DK2BADhoeotPk",
		"zzzzzzzzzzzzzzzzzzzzzz",
		"e3b7fc2ae9447bbec37a13bf916e3cf6",
		"f47ac10b-58cc-4372-a567-0e02b2c3d479",
	} {
		once := Canonical(s)
		if twice := Canonical(once); twice != once {
			t.Errorf("Canonical not idempotent for %q: %q -> %q", s, once, twice)
		}
	}
}

func TestEncode(t *testing.T) {
	if got := Encode([16]byte{}); got != "0000000000000000000000" {
		t.Errorf("Encode(zero) = %q", got)
	}
	allFF := [16]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	if got := Encode(allFF); got != "7N42dgm5tFLK9N8MT7fHC7" {
		t.Errorf("Encode(allFF) = %q", got)
	}
}
