package parser

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"
)

func TestCP932TableRepresentativeMappings(t *testing.T) {
	tests := []struct {
		name string
		code uint16
		want rune
	}{
		{name: "JIS punctuation", code: 0x8140, want: 0x3000},
		{name: "JIS hiragana", code: 0x82a0, want: 0x3042},
		{name: "NEC special character", code: 0x8740, want: 0x2460},
		{name: "NEC-selected IBM extension", code: 0xed40, want: 0x7e8a},
		{name: "IBM extension", code: 0xfa40, want: 0x2170},
		{name: "user-defined first", code: 0xf040, want: 0xe000},
		{name: "user-defined last", code: 0xf9fc, want: 0xe757},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeCP932([]byte{byte(tt.code >> 8), byte(tt.code)}, 1)
			if err != nil {
				t.Fatalf("DecodeCP932(%#x): %v", tt.code, err)
			}
			decoded := []rune(got)
			if len(decoded) != 1 || decoded[0] != tt.want {
				t.Errorf("DecodeCP932(%#x) = %q, want %U", tt.code, got, tt.want)
			}
		})
	}
}

func TestCP932TableUnassignedRanges(t *testing.T) {
	for _, code := range []uint16{0x8540, 0xeb40, 0xec40, 0xef40, 0x9873} {
		if _, err := DecodeCP932([]byte{byte(code >> 8), byte(code)}, 1); err == nil {
			t.Errorf("DecodeCP932(%#x) unexpectedly succeeded", code)
		}
	}
}

func TestCP932TableIntegrity(t *testing.T) {
	if len(cp932Blocks) != 55 {
		t.Errorf("table blocks = %d, want 55", len(cp932Blocks))
	}
	for lead := range cp932Blocks {
		if !(lead >= 0x81 && lead <= 0x9f || lead >= 0xe0 && lead <= 0xfc) {
			t.Errorf("table has invalid lead byte %#x", lead)
		}
	}

	h := sha256.New()
	mapped := 0
	var encoded [4]byte
	for _, lead := range cp932LeadBytes() {
		block := cp932Blocks[lead]
		if block != nil && len(block) != trailSlotsForTest {
			t.Fatalf("lead %#x block length = %d, want %d", lead, len(block), trailSlotsForTest)
		}
		for trail := 0x40; trail <= 0xfc; trail++ {
			var point rune
			if block != nil {
				point = block[trail-0x40]
			}
			if point != 0 {
				mapped++
			}
			binary.BigEndian.PutUint32(encoded[:], uint32(point))
			h.Write(encoded[:])
		}
	}
	if mapped != 9604 {
		t.Errorf("mapped double-byte codes = %d, want 9604", mapped)
	}
	const wantDigest = "e4ee01b616575bfe2d8c49feeaa1b41abddc021aecbc73e80b7d64d376d5e40d"
	if got := fmt.Sprintf("%x", h.Sum(nil)); got != wantDigest {
		t.Errorf("table digest = %s, want %s", got, wantDigest)
	}
}

const trailSlotsForTest = 0xfc - 0x40 + 1

func cp932LeadBytes() []byte {
	leads := make([]byte, 0, 60)
	for lead := 0x81; lead <= 0x9f; lead++ {
		leads = append(leads, byte(lead))
	}
	for lead := 0xe0; lead <= 0xfc; lead++ {
		leads = append(leads, byte(lead))
	}
	return leads
}
