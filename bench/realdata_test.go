package bench_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

type databaseSpec struct {
	dictionary dictionary.Type
	path       string
	queries    []string
}

func BenchmarkRealSearch(b *testing.B) {
	specs := configuredDatabases()
	if len(specs) == 0 {
		b.Skip("set EJQUICK_BENCH_EIJI_DB or EJQUICK_BENCH_WAEI_DB")
	}
	ctx := context.Background()
	for _, spec := range specs {
		repo, err := search.OpenRepository(spec.path, spec.dictionary)
		if err != nil {
			b.Fatalf("open %s benchmark database: %v", spec.dictionary, err)
		}
		service, err := search.NewService(repo, 50)
		if err != nil {
			repo.Close()
			b.Fatal(err)
		}
		b.Cleanup(func() { repo.Close() })

		for i, query := range spec.queries {
			name := fmt.Sprintf("%s/query_%02d_%d_runes", spec.dictionary, i+1, utf8.RuneCountInString(query))
			b.Run(name, func(b *testing.B) {
				if _, err := service.Search(ctx, query); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				for b.Loop() {
					if _, err := service.Search(ctx, query); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkRealOpenRepository(b *testing.B) {
	specs := configuredDatabases()
	if len(specs) == 0 {
		b.Skip("set EJQUICK_BENCH_EIJI_DB or EJQUICK_BENCH_WAEI_DB")
	}
	for _, spec := range specs {
		b.Run(spec.dictionary.String(), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				repo, err := search.OpenRepository(spec.path, spec.dictionary)
				if err != nil {
					b.Fatal(err)
				}
				if err := repo.Close(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func configuredDatabases() []databaseSpec {
	configs := []struct {
		dictionary dictionary.Type
		dbEnv      string
		queryEnv   string
		defaults   string
	}{
		{dictionary.Eiji, "EJQUICK_BENCH_EIJI_DB", "EJQUICK_BENCH_EIJI_QUERIES", "e|en|eng"},
		{dictionary.Waei, "EJQUICK_BENCH_WAEI_DB", "EJQUICK_BENCH_WAEI_QUERIES", "日|日本|日本語"},
	}
	var specs []databaseSpec
	for _, cfg := range configs {
		path := os.Getenv(cfg.dbEnv)
		if path == "" {
			continue
		}
		rawQueries := os.Getenv(cfg.queryEnv)
		if rawQueries == "" {
			rawQueries = cfg.defaults
		}
		var queries []string
		for _, query := range strings.Split(rawQueries, "|") {
			if query = strings.TrimSpace(query); query != "" {
				queries = append(queries, query)
			}
		}
		specs = append(specs, databaseSpec{
			dictionary: cfg.dictionary,
			path:       path,
			queries:    queries,
		})
	}
	return specs
}
