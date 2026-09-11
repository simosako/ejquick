package normalize_test

import (
	"testing"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/normalize"
)

func TestNormalizeEiji(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercase passthrough", "english", "english"},
		{"uppercase to lower", "English", "english"},
		{"all uppercase", "ENGLISH", "english"},
		{"trim surrounding spaces", "  english ", "english"},
		{"inner spaces kept", "take care", "take care"},
		{"tab trimmed", "\tenglish\t", "english"},
		{"sharp s folds to ss", "STRASSE", "strasse"},
		{"german lower stays", "straße", "strasse"},
		{"greek sigma folds context-independently", "ΟΔΟΣ", "οδοσ"},
		{"greek final sigma folds to sigma", "ΟΔΟΣ"[:0] + "οδος", "οδοσ"},
		{"accented char kept", "café", "cafe"[:0] + "café"},
		{"combining accent composed", "cafe\u0301", "café"},
		{"hyphen kept", "state-of-the-art", "state-of-the-art"},
		{"digits kept", "mp3", "mp3"},
		{"empty string", "", ""},
		{"only spaces", "   ", ""},
		{"leading structural marker stripped", "■english", "english"},
		{"marker followed by space", "■ english ", "english"},
		{"inner marker kept", "a■b", "a■b"},
		{"marker only", "■", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalize.MustNormalize(dictionary.Eiji, tt.in)
			if got != tt.want {
				t.Errorf("eiji normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeWaei(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"hiragana kept", "ねこ", "ねこ"},
		{"katakana kept", "ネコ", "ネコ"},
		{"kanji kept", "日本語", "日本語"},
		{"full-width digit to ascii", "１", "1"},
		{"full-width letters to ascii", "ＡＢＣ", "abc"},
		{"half-width katakana to full", "ｶﾞ", "ガ"},
		{"mixed width normalized", "Ｔｏｋｙｏ", "tokyo"},
		{"trim spaces", " ねこ ", "ねこ"},
		{"full-width space trimmed", "\u3000ねこ\u3000", "ねこ"},
		{"ascii passthrough", "hello", "hello"},
		{"ascii upper folds", "HELLO", "hello"},
		{"kanji digit is not converted", "一", "一"},
		{"long vowel mark kept", "ー", "ー"},
		{"leading structural marker stripped", "■ねこ", "ねこ"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalize.MustNormalize(dictionary.Waei, tt.in)
			if got != tt.want {
				t.Errorf("waei normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeUnknownType(t *testing.T) {
	if _, err := normalize.Normalize(dictionary.Type("xxx"), "a"); err == nil {
		t.Fatal("expected error for unknown dictionary type")
	}
}

func TestEmptyAfter(t *testing.T) {
	if !normalize.EmptyAfter(dictionary.Eiji, "  ") {
		t.Error("spaces only should be empty after normalization")
	}
	if normalize.EmptyAfter(dictionary.Eiji, "a") {
		t.Error("non-empty string reported as empty")
	}
	if !normalize.EmptyAfter(dictionary.Waei, "\u3000") {
		t.Error("ideographic space only should be empty after normalization")
	}
}

// TestEijiWaeiAgreement checks that ASCII inputs normalize identically in
// both dictionaries so that dictionary switching yields consistent keys for
// latin input.
func TestEijiWaeiAgreement(t *testing.T) {
	for _, s := range []string{"English", " TAKE CARE ", "mp3"} {
		e := normalize.MustNormalize(dictionary.Eiji, s)
		w := normalize.MustNormalize(dictionary.Waei, s)
		if e != w {
			t.Errorf("normalization mismatch for %q: eiji=%q waei=%q", s, e, w)
		}
	}
}
