// Package fold provides diacritic-insensitive string matching.
//
// SQLite LIKE does byte comparison and FTS folding only applies to the FTS
// index, so every non-FTS search path (artist:/album: prefix search, genre
// filtering, play-query resolution) needs query-side folding. FoldString
// maps both needle and haystack into a comparable ASCII-folded,
// lower-cased form. Scope mirrors FTS remove_diacritics=1 (Latin-focused);
// non-Latin scripts pass through unchanged.
package fold

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var foldChain = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// String maps s to its folded form: diacritics stripped, lower-cased.
// Idempotent: String(String(s)) == String(s).
func String(s string) string {
	out, _, err := transform.String(foldChain, s)
	if err != nil {
		return strings.ToLower(s)
	}
	return strings.ToLower(out)
}

// Equal reports whether a and b match ignoring diacritics and case.
func Equal(a, b string) bool {
	return String(a) == String(b)
}

// Contains reports whether b's folded form is a substring of a's folded form.
func Contains(a, b string) bool {
	return strings.Contains(String(a), String(b))
}
