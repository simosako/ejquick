// Package parser decodes the CP932 (Windows-31J) dictionary TXT files,
// removes the source headword marker, and splits each physical line around
// the single ASCII separator " : ".
package parser

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

const (
	// HeadwordMarker marks the start of a headword in the source TXT. It is
	// source syntax and is not part of the parsed headword.
	HeadwordMarker = "■"
	// Separator separates headword and body in one physical line. It must
	// occur exactly once, and both sides must be non-empty, for a line to be
	// valid.
	Separator = " : "
)

// Entry is a parsed dictionary entry from one valid physical line.
type Entry struct {
	// LineNo is the 1-based physical line number in the source TXT.
	// It becomes the entries.id in the database, keeping gaps for
	// skipped lines.
	LineNo   int64
	Headword string
	Body     string
}

// SkipInfo describes a malformed physical line that is not registered.
type SkipInfo struct {
	LineNo int64
	Reason string
}

// Line is the result of reading one physical line: exactly one of Entry or
// Skip is non-nil.
type Line struct {
	LineNo int64
	Entry  *Entry
	Skip   *SkipInfo
}

// ParseLine validates one decoded line with line endings removed and splits
// it into headword and body. ok is false with a human-readable reason when
// the line is malformed.
func ParseLine(line string) (headword, body string, ok bool, reason string) {
	if line == "" {
		return "", "", false, "empty line"
	}
	first := strings.Index(line, Separator)
	if first < 0 {
		return "", "", false, "missing separator"
	}
	if rest := line[first+len(Separator):]; strings.Index(rest, Separator) >= 0 {
		return "", "", false, "multiple separators"
	}
	headword = line[:first]
	body = line[first+len(Separator):]
	if headword == "" {
		return "", "", false, "empty headword"
	}
	var found bool
	headword, found = strings.CutPrefix(headword, HeadwordMarker)
	if !found {
		return "", "", false, "missing headword marker"
	}
	if headword == "" {
		return "", "", false, "empty headword"
	}
	if body == "" {
		return "", "", false, "empty body"
	}
	return headword, body, true, ""
}

// Reader reads a CP932-encoded TXT stream line by line and yields parsed
// entries. Physical line numbers start at 1 and increase by one per input
// line regardless of validity, so ids stay aligned with the source file.
type Reader struct {
	scanner *bufio.Scanner
	lineNo  int64
}

// NewReader returns a Reader that decodes r as CP932. Decoding is strict:
// the first invalid byte sequence is a fatal error for the whole
// conversion.
func NewReader(r io.Reader) *Reader {
	sc := bufio.NewScanner(r)
	// Dictionary lines can be long; allow up to 1 MiB per physical line.
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	return &Reader{scanner: sc}
}

// Next returns the next physical line. io.EOF signals the end of input.
// Any other non-nil error is a decode error: the caller must stop the
// conversion and report it.
func (r *Reader) Next() (Line, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return Line{}, err
		}
		return Line{}, io.EOF
	}
	r.lineNo++
	// bufio.ScanLines already strips the trailing "\r\n" / "\n".
	decoded, err := DecodeCP932(r.scanner.Bytes(), r.lineNo)
	if err != nil {
		return Line{}, err
	}
	head, body, ok, reason := ParseLine(decoded)
	if ok {
		return Line{
			LineNo: r.lineNo,
			Entry:  &Entry{LineNo: r.lineNo, Headword: head, Body: body},
		}, nil
	}
	return Line{
		LineNo: r.lineNo,
		Skip:   &SkipInfo{LineNo: r.lineNo, Reason: reason},
	}, nil
}

// ErrInvalidCP932 reports input bytes that are not valid CP932.
type ErrInvalidCP932 struct {
	LineNo     int64
	ByteOffset int
	Detail     string
}

func (e *ErrInvalidCP932) Error() string {
	return fmt.Sprintf("invalid CP932 data at line %d offset %d: %s", e.LineNo, e.ByteOffset, e.Detail)
}

// DecodeCP932 decodes one physical line of CP932 bytes, rejecting both
// structurally invalid sequences and pairs unassigned in the official
// Windows-31J mapping table instead of substituting U+FFFD.
func DecodeCP932(p []byte, lineNo int64) (string, error) {
	var b strings.Builder
	b.Grow(len(p)*3/2 + 8)
	for i := 0; i < len(p); {
		c := p[i]
		switch {
		case c <= 0x7F:
			b.WriteByte(c)
			i++
		case c >= 0xA1 && c <= 0xDF:
			// Half-width katakana: U+FF61..U+FF9F.
			b.WriteRune(0xFF61 + rune(c) - 0xA1)
			i++
		default:
			// Lead byte (0x81-0x9F, 0xE0-0xFC).
			block, ok := cp932Blocks[c]
			if !ok || i+1 >= len(p) {
				return "", &ErrInvalidCP932{LineNo: lineNo, ByteOffset: i, Detail: "invalid byte sequence"}
			}
			t := p[i+1]
			if t < 0x40 || t == 0x7F || t > 0xFC {
				return "", &ErrInvalidCP932{LineNo: lineNo, ByteOffset: i, Detail: "invalid byte sequence"}
			}
			r := block[t-0x40]
			if r == 0 {
				return "", &ErrInvalidCP932{LineNo: lineNo, ByteOffset: i, Detail: "unmapped code point"}
			}
			b.WriteRune(r)
			i += 2
		}
	}
	return b.String(), nil
}
