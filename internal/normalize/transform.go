package normalize

import (
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// nfc applies Unicode Normalization Form C.
func nfc(s string) string {
	if !utf8.ValidString(s) {
		return s
	}
	if norm.NFC.IsNormalString(s) {
		return s
	}
	return norm.NFC.String(s)
}

// nfkc applies Unicode Normalization Form KC (compatibility decomposition
// followed by canonical composition). This maps full-width ASCII and
// half-width katakana to their canonical equivalents.
func nfkc(s string) string {
	if !utf8.ValidString(s) {
		return s
	}
	if norm.NFKC.IsNormalString(s) {
		return s
	}
	return norm.NFKC.String(s)
}

// fold applies Unicode full case folding.
func fold(s string) string {
	return folder.String(s)
}
