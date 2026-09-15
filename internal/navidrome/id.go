// Package navidrome reproduces Navidrome's 0.64 id migration so must can
// translate ids cached before the upgrade to their canonical post-0.64 form.
//
// Navidrome 0.64 (PR #5824) re-encodes every internal id to a 22-character
// zero-padded base62 representation of a 128-bit value. The migration is
// deterministic: the same pre-0.64 id always maps to the same new id. This
// package is a faithful port of Navidrome's canonicalID + model/id.Encode,
// verified against Navidrome's own golden test vectors.
package navidrome

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/big"
)

// Canonical maps a pre-0.64 Navidrome id to its canonical 0.64 form. Ids that
// are already canonical, and shapes the migration leaves untouched (share ids,
// truncated ids, non-base62 junk), are returned unchanged. It is idempotent.
//
// The three transformed shapes are, matching Navidrome exactly:
//   - 22-char base62 values that overflow 128 bits are remapped through md5 of
//     the id string;
//   - 32-hex legacy MD5 ids are re-encoded value-preserving;
//   - 36-char legacy playlist UUIDs are re-encoded value-preserving.
func Canonical(s string) string {
	switch len(s) {
	case 22:
		v, ok := new(big.Int).SetString(s, 62)
		if !ok || v.Sign() < 0 || v.BitLen() <= 128 {
			return s
		}
		return Encode(md5.Sum([]byte(s)))
	case 32:
		b, err := hex.DecodeString(s)
		if err != nil {
			return s
		}
		return Encode([16]byte(b))
	case 36:
		if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
			return s
		}
		b, err := hex.DecodeString(s[:8] + s[9:13] + s[14:18] + s[19:23] + s[24:])
		if err != nil {
			return s
		}
		return Encode([16]byte(b))
	}
	return s
}

// Encode renders 16 bytes as the canonical 22-char zero-padded base62 id.
func Encode(b [16]byte) string {
	return fmt.Sprintf("%022s", new(big.Int).SetBytes(b[:]).Text(62))
}
