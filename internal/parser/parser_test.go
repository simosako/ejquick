package parser_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/simosako/ejquick/internal/parser"
)

// CP932 byte helpers: build raw lines without relying on the Go source
// encoding.
func b(parts ...byte) []byte { return parts }

func TestParseLineValid(t *testing.T) {
	tests := []struct {
		name string
		line string
		head string
		body string
	}{
		{"simple", "■english : a language", "english", "a language"},
		{"spaces in body", "■take care : be careful", "take care", "be careful"},
		{"head with symbols", "■A/B : either", "A/B", "either"},
		{"body with colon glued", "■x : note: it", "x", "note: it"},
		{"head with fullwidth colon", "■英：語 : body", "英：語", "body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			head, body, ok, reason := parser.ParseLine(tt.line)
			if !ok {
				t.Fatalf("ParseLine(%q) not ok: %s", tt.line, reason)
			}
			if head != tt.head || body != tt.body {
				t.Errorf("ParseLine(%q) = (%q, %q), want (%q, %q)", tt.line, head, body, tt.head, tt.body)
			}
		})
	}
}

func TestParseLineMalformed(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		reason string
	}{
		{"empty line", "", "empty line"},
		{"missing separator", "■headword body", "missing separator"},
		{"colon without spaces", "■head:body", "missing separator"},
		{"only left space", "■head :body", "missing separator"},
		{"only right space", "■head: body", "missing separator"},
		{"multiple separators", "■a : b : c", "multiple separators"},
		{"empty headword", " : body", "empty headword"},
		{"marker only headword", "■ : body", "empty headword"},
		{"missing headword marker", "head : body", "missing headword marker"},
		{"empty body", "■head : ", "empty body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, ok, reason := parser.ParseLine(tt.line)
			if ok {
				t.Fatalf("ParseLine(%q) unexpectedly ok", tt.line)
			}
			if reason != tt.reason {
				t.Errorf("ParseLine(%q) reason = %q, want %q", tt.line, reason, tt.reason)
			}
		})
	}
}

// readAll drains a Reader, collecting valid entries and skips.
func readAll(t *testing.T, r *parser.Reader) (entries []parser.Entry, skips []parser.SkipInfo, err error) {
	t.Helper()
	for {
		line, e := r.Next()
		if e != nil {
			if errors.Is(e, io.EOF) {
				return entries, skips, nil
			}
			return entries, skips, e
		}
		if line.Entry != nil {
			entries = append(entries, *line.Entry)
		}
		if line.Skip != nil {
			skips = append(skips, *line.Skip)
		}
	}
}

// CP932 samples. "■" is 0x81 0xA1, "＜" is 0x81 0x97, "→" is 0x81 0xA8,
// "ねこ" is 0x82 0xCB 0x82 0xB1, "ｶﾞ" is 0xB6 0xDE.
func TestReaderCP932Lines(t *testing.T) {
	// Line 1: "■! : body1"
	// Line 2: "■ねこ : cat"
	// Line 3: malformed (missing separator)
	// Line 4: valid again to check line numbering over a skip.
	var buf bytes.Buffer
	buf.Write([]byte{0x81, 0xA1, '!', ' ', ':', ' ', 'b', 'o', 'd', 'y', '1', '\r', '\n'})
	buf.Write([]byte{0x81, 0xA1, 0x82, 0xCB, 0x82, 0xB1, ' ', ':', ' ', 'c', 'a', 't', '\n'})
	buf.Write([]byte{0x81, 0xA1})
	buf.WriteString("no separator line\n")
	buf.Write([]byte{0x81, 0xA1})
	buf.WriteString("dog : animal\n")

	entries, skips, err := readAll(t, parser.NewReader(&buf))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(entries), entries)
	}
	if len(skips) != 1 {
		t.Fatalf("got %d skips, want 1: %+v", len(skips), skips)
	}
	if entries[0].LineNo != 1 || entries[0].Headword != "!" || entries[0].Body != "body1" {
		t.Errorf("entry 1 mismatch: %+v", entries[0])
	}
	if entries[1].LineNo != 2 || entries[1].Headword != "ねこ" || entries[1].Body != "cat" {
		t.Errorf("entry 2 mismatch: %+v", entries[1])
	}
	if entries[2].LineNo != 4 || entries[2].Headword != "dog" {
		t.Errorf("entry 3 mismatch: %+v", entries[2])
	}
	if skips[0].LineNo != 3 || skips[0].Reason != "missing separator" {
		t.Errorf("skip mismatch: %+v", skips[0])
	}
}

func TestReaderEmptyLastLine(t *testing.T) {
	// A trailing newline after the last entry must not count as a line.
	buf := strings.NewReader("\x81\xa1a : b\n")
	entries, skips, err := readAll(t, parser.NewReader(buf))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if len(entries) != 1 || len(skips) != 0 {
		t.Fatalf("unexpected result: entries=%d skips=%d", len(entries), len(skips))
	}
}

func TestReaderEmptyLineWithinFile(t *testing.T) {
	buf := strings.NewReader("\x81\xa1a : b\n\n\x81\xa1c : d\n")
	entries, skips, err := readAll(t, parser.NewReader(buf))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if len(entries) != 2 || len(skips) != 1 {
		t.Fatalf("entries=%d skips=%d", len(entries), len(skips))
	}
	if skips[0].LineNo != 2 || skips[0].Reason != "empty line" {
		t.Errorf("skip mismatch: %+v", skips[0])
	}
	if entries[1].LineNo != 3 {
		t.Errorf("second entry line = %d, want 3", entries[1].LineNo)
	}
}

func TestReaderDuplicateHeadwordsKept(t *testing.T) {
	buf := strings.NewReader("\x81\xa1same : first\n\x81\xa1same : second\n")
	entries, _, err := readAll(t, parser.NewReader(buf))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("duplicates must be kept: got %d", len(entries))
	}
	if entries[0].Body != "first" || entries[1].Body != "second" {
		t.Errorf("duplicate bodies mismatch: %+v", entries)
	}
}

func TestDecodeCP932InvalidSequences(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
	}{
		{"invalid lead byte 0x80", b(0x80, 0x40)},
		{"invalid lead byte 0xA0", b(0xA0, 0x40)},
		{"invalid lead byte 0xFD", b(0xFD, 0x40)},
		{"truncated pair", b(0x82)},
		{"invalid trail 0x3F", b(0x82, 0x3F)},
		{"invalid trail 0x7F", b(0x82, 0x7F)},
		{"invalid trail 0xFD", b(0x82, 0xFD)},
		{"bare 0x80", b('a', 0x80)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.DecodeCP932(tt.raw, 1)
			if err == nil {
				t.Fatalf("DecodeCP932(%v) unexpectedly succeeded", tt.raw)
			}
			var inv *parser.ErrInvalidCP932
			if !errors.As(err, &inv) {
				t.Fatalf("error is not ErrInvalidCP932: %v", err)
			}
			if inv.ByteOffset < 0 {
				t.Errorf("expected non-negative byte offset, got %d", inv.ByteOffset)
			}
		})
	}
}

// The well-known unmapped CP932 pair 0x98 0x73 must be rejected rather
// than silently turned into U+FFFD.
func TestDecodeCP932UnmappedPair(t *testing.T) {
	_, err := parser.DecodeCP932([]byte{0x98, 0x73}, 7)
	if err == nil {
		t.Fatal("unmapped CP932 pair unexpectedly decoded")
	}
	var inv *parser.ErrInvalidCP932
	if !errors.As(err, &inv) {
		t.Fatalf("error is not ErrInvalidCP932: %v", err)
	}
	if inv.LineNo != 7 {
		t.Errorf("line number = %d, want 7", inv.LineNo)
	}
}

func TestReaderStopsOnDecodeError(t *testing.T) {
	buf := bytes.NewReader([]byte{0x81, 0xA1, 'a', ' ', ':', ' ', 'b', '\n', 0x82, '\n', 0x81, 0xA1, 'c', ' ', ':', ' ', 'd', '\n'})
	entries, _, err := readAll(t, parser.NewReader(buf))
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
	var inv *parser.ErrInvalidCP932
	if !errors.As(err, &inv) {
		t.Fatalf("error is not ErrInvalidCP932: %v", err)
	}
	if inv.LineNo != 2 {
		t.Errorf("error line = %d, want 2", inv.LineNo)
	}
	// Only the first line was yielded before the fatal error.
	if len(entries) != 1 {
		t.Errorf("entries before failure = %d, want 1", len(entries))
	}
}

func TestReaderLongLine(t *testing.T) {
	long := strings.Repeat("x", 300*1024)
	buf := strings.NewReader("\x81\xa1" + long + " : body\n")
	entries, _, err := readAll(t, parser.NewReader(buf))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if len(entries) != 1 || entries[0].Headword != long {
		t.Fatalf("long line not read correctly")
	}
}
