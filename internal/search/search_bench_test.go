package search_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

func BenchmarkSearch(b *testing.B) {
	path := buildArtificialBenchmarkDB(b, 20_000)
	repo, err := search.OpenRepository(path, dictionary.Eiji)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { repo.Close() })
	service, err := search.NewService(repo, 50)
	if err != nil {
		b.Fatal(err)
	}

	cases := []struct {
		name  string
		query string
	}{
		{name: "prefix_1_rune", query: "a"},
		{name: "prefix_2_runes", query: "al"},
		{name: "prefix_full_pool", query: "alpha"},
		{name: "substring", query: "needle"},
		{name: "mixed_prefix_substring", query: "term"},
	}
	ctx := context.Background()
	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			if _, err := service.Search(ctx, bc.query); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				entries, err := service.Search(ctx, bc.query)
				if err != nil {
					b.Fatal(err)
				}
				if len(entries) == 0 {
					b.Fatalf("artificial query %q returned no results", bc.query)
				}
			}
		})
	}
}

func BenchmarkOpenRepository(b *testing.B) {
	path := buildArtificialBenchmarkDB(b, 20_000)
	b.ReportAllocs()
	for b.Loop() {
		repo, err := search.OpenRepository(path, dictionary.Eiji)
		if err != nil {
			b.Fatal(err)
		}
		if err := repo.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func buildArtificialBenchmarkDB(b *testing.B, rows int) string {
	b.Helper()
	dir := b.TempDir()
	input := filepath.Join(dir, "EIJIRO-BENCH-1-0.TXT")
	f, err := os.Create(input)
	if err != nil {
		b.Fatal(err)
	}
	for i := range rows {
		var headword string
		switch {
		case i < 20:
			headword = fmt.Sprintf("term%06d", i)
		case i%3 == 0:
			headword = fmt.Sprintf("alpha%06d", i)
		case i%3 == 1:
			headword = fmt.Sprintf("word%06dneedle", i)
		default:
			headword = fmt.Sprintf("word%06dterm", i)
		}
		if _, err := fmt.Fprintf(f, "\x81\xa1%s : artificial definition %06d\r\n", headword, i); err != nil {
			f.Close()
			b.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		b.Fatal(err)
	}
	output := filepath.Join(dir, "eiji.sqlite3")
	if _, err := builder.Run(builder.Options{
		Type: dictionary.Eiji, Input: input, Output: output, Progress: io.Discard,
	}); err != nil {
		b.Fatal(err)
	}
	return output
}
