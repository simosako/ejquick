package search

import (
	"context"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/simosako/ejquick/internal/normalize"
)

// Service executes Auto-mode searches against a Repository. Auto mode is
// the only user-facing search mode: queries of one or two runes use the
// B-tree prefix index; queries of three or more runes additionally fill
// the remaining slots with FTS5 substring candidates.
type Service struct {
	repo *Repository
	// maxResults is the result limit, validated to 1..HardMaxResults.
	maxResults int
}

// NewService returns a Service for repo with the given result limit.
// Limits outside 1..HardMaxResults are rejected.
func NewService(repo *Repository, maxResults int) (*Service, error) {
	if maxResults < 1 || maxResults > HardMaxResults {
		return nil, fmt.Errorf("max results %d out of range 1..%d", maxResults, HardMaxResults)
	}
	return &Service{repo: repo, maxResults: maxResults}, nil
}

// MaxResults returns the configured result limit.
func (s *Service) MaxResults() int { return s.maxResults }

// Close releases the underlying repository.
func (s *Service) Close() error { return s.repo.Close() }

// Search runs the Auto search for the raw query string. The query is
// normalized with the repository's dictionary rules. An empty (or
// normalization-empty) query returns no rows without touching the
// database. Results are deterministic for the same database and query.
func (s *Service) Search(ctx context.Context, rawQuery string) ([]Entry, error) {
	normQuery, err := normalize.Normalize(s.repo.Dictionary(), rawQuery)
	if err != nil {
		return nil, err
	}
	if normQuery == "" {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Prefix pool: exact + prefix matches, ranked per design 8.6.
	pool, err := s.repo.queryPrefixPool(ctx, normQuery, poolLimitForPrefix(s.maxResults))
	if err != nil {
		return nil, fmt.Errorf("prefix query: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prefixRanked := rankPrefixPool(normQuery, pool)
	if len(prefixRanked) > s.maxResults {
		prefixRanked = prefixRanked[:s.maxResults]
	}

	results := prefixRanked
	remaining := s.maxResults - len(results)
	// Only queries of three or more runes can hit the trigram index.
	if remaining <= 0 || utf8.RuneCountInString(normQuery) < 3 {
		return toEntries(results), nil
	}

	// Substring pool fills the remaining slots.
	subPool, err := s.repo.querySubstringPool(ctx, normQuery, candidateLimitForSubstring(remaining))
	if err != nil {
		return nil, fmt.Errorf("substring query: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	seen := make(map[int64]struct{}, len(results))
	for _, c := range results {
		seen[c.id] = struct{}{}
	}
	filtered := subPool[:0:0]
	for _, c := range subPool {
		if _, dup := seen[c.id]; dup {
			continue
		}
		filtered = append(filtered, c)
	}
	subRanked := rankSubstringPool(normQuery, filtered)
	take := remaining
	if len(subRanked) < take {
		take = len(subRanked)
	}
	results = append(results, subRanked[:take]...)
	return toEntries(results), nil
}

// toEntries converts internal candidate rows into result entries.
func toEntries(rows []candidateRow) []Entry {
	out := make([]Entry, len(rows))
	for i, r := range rows {
		out[i] = Entry{ID: r.id, Headword: r.headword, Body: r.body}
	}
	return out
}

// rankPrefixPool orders the prefix pool: exact matches first, then
// shorter headwords, then dictionary order, then id. All comparisons are
// in rune units.
func rankPrefixPool(normQuery string, pool []candidateRow) []candidateRow {
	type key struct {
		exact   int
		runes   int
		norm    string
		id      int64
	}
	keys := make([]key, len(pool))
	for i, c := range pool {
		k := key{runes: utf8.RuneCountInString(c.norm), norm: c.norm, id: c.id}
		if c.norm == normQuery {
			k.exact = 0
		} else {
			k.exact = 1
		}
		keys[i] = k
	}
	idx := make([]int, len(pool))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		ka, kb := keys[idx[a]], keys[idx[b]]
		if ka.exact != kb.exact {
			return ka.exact < kb.exact
		}
		if ka.runes != kb.runes {
			return ka.runes < kb.runes
		}
		if ka.norm != kb.norm {
			return ka.norm < kb.norm
		}
		return ka.id < kb.id
	})
	out := make([]candidateRow, len(idx))
	for i, j := range idx {
		out[i] = pool[j]
	}
	return out
}

// rankSubstringPool orders the substring pool by earliest match position
// (rune index), then shorter headword, then dictionary order, then id.
func rankSubstringPool(normQuery string, pool []candidateRow) []candidateRow {
	type key struct {
		pos   int
		runes int
		norm  string
		id    int64
	}
	keys := make([]key, len(pool))
	for i, c := range pool {
		keys[i] = key{
			pos:   runeIndex(c.norm, normQuery),
			runes: utf8.RuneCountInString(c.norm),
			norm:  c.norm,
			id:    c.id,
		}
	}
	idx := make([]int, len(pool))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		ka, kb := keys[idx[a]], keys[idx[b]]
		if ka.pos != kb.pos {
			return ka.pos < kb.pos
		}
		if ka.runes != kb.runes {
			return ka.runes < kb.runes
		}
		if ka.norm != kb.norm {
			return ka.norm < kb.norm
		}
		return ka.id < kb.id
	})
	out := make([]candidateRow, len(idx))
	for i, j := range idx {
		out[i] = pool[j]
	}
	return out
}

// runeIndex returns the rune index of the first occurrence of sub in s,
// or -1 when absent.
func runeIndex(s, sub string) int {
	if sub == "" {
		return 0
	}
	first := []rune(sub)[0]
	sr := []rune(s)
	subR := []rune(sub)
	for i := 0; i+len(subR) <= len(sr); i++ {
		if sr[i] != first {
			continue
		}
		match := true
		for j := 1; j < len(subR); j++ {
			if sr[i+j] != subR[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
