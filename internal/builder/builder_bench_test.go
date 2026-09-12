package builder_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/dictionary"
)

func BenchmarkBuild(b *testing.B) {
	cases := []struct {
		name    string
		rows    int
		compact bool
	}{
		{name: "rows_100", rows: 100},
		{name: "rows_10000", rows: 10_000},
		{name: "rows_1000_compact", rows: 1_000, compact: true},
	}

	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			data := artificialBuilderData(bc.rows)
			dir := b.TempDir()
			input := filepath.Join(dir, "EIJIRO-BENCH-1-0.TXT")
			output := filepath.Join(dir, "eiji.sqlite3")
			if err := os.WriteFile(input, data, 0o644); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()

			var stats builder.Stats
			for b.Loop() {
				var err error
				stats, err = builder.Run(builder.Options{
					Type:     dictionary.Eiji,
					Input:    input,
					Output:   output,
					Force:    true,
					Compact:  bc.compact,
					Progress: io.Discard,
				})
				if err != nil {
					b.Fatal(err)
				}
			}
			if stats.Entries != int64(bc.rows) {
				b.Fatalf("entries = %d, want %d", stats.Entries, bc.rows)
			}
			b.ReportMetric(float64(stats.DBSize), "db-bytes")
		})
	}
}

func artificialBuilderData(rows int) []byte {
	var out strings.Builder
	out.Grow(rows * 64)
	for i := range rows {
		fmt.Fprintf(&out, "\x81\xa1entry%06d : artificial definition %06d\r\n", i, i)
	}
	return []byte(out.String())
}
