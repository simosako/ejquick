// Command gencp932 generates the strict CP932 double-byte decoding table.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	mappingURL = "https://www.unicode.org/Public/MAPPINGS/VENDORS/MICSFT/WINDOWS/CP932.TXT"
	// The source file identifies itself as Microsoft table version 2.01,
	// dated 1998-04-15, for Unicode 2.0.
	mappingVersion = "2.01 (1998-04-15)"
	mappingSHA256  = "c9bc0b0cd42e0fbcb82a09635bb5abed86afbdd4abc9e76fa5716638217cb59f"

	firstTrail                  = 0x40
	lastTrail                   = 0xfc
	trailSlots                  = lastTrail - firstTrail + 1
	expectedSourceMappings      = 7724
	expectedUserDefinedMappings = 1880
)

func main() {
	input := flag.String("input", "", "local CP932.TXT path (download when empty)")
	output := flag.String("output", "", "generated Go file path (required)")
	flag.Parse()
	if *output == "" {
		fmt.Fprintln(os.Stderr, "gencp932: -output is required")
		os.Exit(2)
	}
	if err := run(*input, *output); err != nil {
		fmt.Fprintf(os.Stderr, "gencp932: %v\n", err)
		os.Exit(1)
	}
}

func run(input, output string) error {
	data, err := loadMapping(input)
	if err != nil {
		return err
	}
	if err := verifyMapping(data); err != nil {
		return err
	}
	blocks, count, err := parseMapping(data)
	if err != nil {
		return err
	}
	if count != expectedSourceMappings {
		return fmt.Errorf("source contains %d double-byte mappings, want %d", count, expectedSourceMappings)
	}
	puaCount, err := addUserDefinedMappings(blocks)
	if err != nil {
		return err
	}
	if puaCount != expectedUserDefinedMappings {
		return fmt.Errorf("generated %d user-defined mappings, want %d", puaCount, expectedUserDefinedMappings)
	}

	source, err := render(blocks)
	if err != nil {
		return err
	}
	if err := os.WriteFile(output, source, 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func loadMapping(input string) ([]byte, error) {
	if input != "" {
		data, err := os.ReadFile(input)
		if err != nil {
			return nil, fmt.Errorf("read mapping: %w", err)
		}
		return data, nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(mappingURL)
	if err != nil {
		return nil, fmt.Errorf("download mapping: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download mapping: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read downloaded mapping: %w", err)
	}
	return data, nil
}

func verifyMapping(data []byte) error {
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	if got != mappingSHA256 {
		return fmt.Errorf("mapping SHA-256 = %s, want %s", got, mappingSHA256)
	}
	return nil
}

func parseMapping(data []byte) (map[byte][]rune, int, error) {
	blocks := make(map[byte][]rune)
	count := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.SplitN(scanner.Text(), "#", 2)[0]
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		code, err := strconv.ParseUint(fields[0], 0, 16)
		if err != nil {
			return nil, 0, fmt.Errorf("mapping line %d code: %w", lineNo, err)
		}
		point, err := strconv.ParseUint(fields[1], 0, 32)
		if err != nil {
			return nil, 0, fmt.Errorf("mapping line %d code point: %w", lineNo, err)
		}
		if code <= 0xff {
			continue
		}
		lead, trail := byte(code>>8), byte(code)
		if !validLead(lead) || !validTrail(trail) {
			return nil, 0, fmt.Errorf("mapping line %d has unsupported CP932 code %#x", lineNo, code)
		}
		if point == 0 || point > 0x10ffff {
			return nil, 0, fmt.Errorf("mapping line %d has invalid Unicode code point %#x", lineNo, point)
		}
		block := ensureBlock(blocks, lead)
		index := int(trail) - firstTrail
		if block[index] != 0 {
			return nil, 0, fmt.Errorf("mapping line %d duplicates CP932 code %#x", lineNo, code)
		}
		block[index] = rune(point)
		count++
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("scan mapping: %w", err)
	}
	return blocks, count, nil
}

// addUserDefinedMappings fills the Windows-compatible Shift_JIS user-defined
// range omitted by CP932.TXT: 0xF040..0xF9FC maps sequentially to
// U+E000..U+E757, excluding trail byte 0x7F.
func addUserDefinedMappings(blocks map[byte][]rune) (int, error) {
	point := rune(0xe000)
	count := 0
	for lead := byte(0xf0); lead <= 0xf9; lead++ {
		block := ensureBlock(blocks, lead)
		for trail := firstTrail; trail <= lastTrail; trail++ {
			if !validTrail(byte(trail)) {
				continue
			}
			index := trail - firstTrail
			if block[index] != 0 {
				return 0, fmt.Errorf("user-defined CP932 code %#x%02x is already mapped", lead, trail)
			}
			block[index] = point
			point++
			count++
		}
	}
	return count, nil
}

func ensureBlock(blocks map[byte][]rune, lead byte) []rune {
	block := blocks[lead]
	if block == nil {
		block = make([]rune, trailSlots)
		blocks[lead] = block
	}
	return block
}

func validLead(b byte) bool {
	return b >= 0x81 && b <= 0x9f || b >= 0xe0 && b <= 0xfc
}

func validTrail(b byte) bool {
	return b >= firstTrail && b <= lastTrail && b != 0x7f
}

func render(blocks map[byte][]rune) ([]byte, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by tools/gencp932 from Microsoft CP932 table %s; DO NOT EDIT.\n", mappingVersion)
	fmt.Fprintf(&b, "// Source: %s\n", mappingURL)
	fmt.Fprintf(&b, "// Source SHA-256: %s\n\n", mappingSHA256)
	b.WriteString("// The Windows user-defined range is added per the WHATWG Shift_JIS decoder.\n\n")
	b.WriteString("package parser\n\n")
	b.WriteString("// cp932Blocks maps each valid CP932 lead byte (0x81-0x9F, 0xE0-0xFC)\n")
	b.WriteString("// to the runes for trail bytes 0x40..0xFC. A zero entry means the pair\n")
	b.WriteString("// is unassigned in CP932 and is rejected as invalid input.\n")
	b.WriteString("var cp932Blocks = map[byte][]rune{\n")
	for _, lead := range leadBytes() {
		block := blocks[lead]
		if block == nil {
			continue
		}
		fmt.Fprintf(&b, "\t0x%02x: {", lead)
		for i, point := range block {
			if i > 0 {
				b.WriteString(", ")
			}
			if point == 0 {
				b.WriteByte('0')
			} else {
				fmt.Fprintf(&b, "0x%04x", point)
			}
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")

	source, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w", err)
	}
	return source, nil
}

func leadBytes() []byte {
	leads := make([]byte, 0, 60)
	for lead := 0x81; lead <= 0x9f; lead++ {
		leads = append(leads, byte(lead))
	}
	for lead := 0xe0; lead <= 0xfc; lead++ {
		leads = append(leads, byte(lead))
	}
	return leads
}
