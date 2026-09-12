package search

import (
	"fmt"
	"testing"
)

func TestRankPrefixPoolUsesRuneLengthNormAndID(t *testing.T) {
	pool := []candidateRow{
		{id: 5, norm: "語い"},
		{id: 2, norm: "語"},
		{id: 4, norm: "語あ"},
		{id: 1, norm: "語"},
		{id: 3, norm: "語あ"},
		{id: 6, norm: "語辞書"},
	}
	got := candidateIDs(rankPrefixPool("語", pool))
	want := []int64{1, 2, 3, 4, 5, 6}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("ranked IDs = %v, want %v", got, want)
	}
}

func TestRankSubstringPoolUsesRunePositionLengthNormAndID(t *testing.T) {
	pool := []candidateRow{
		{id: 5, norm: "あ検索語"},
		{id: 1, norm: "ああ検索"},
		{id: 4, norm: "い検索語"},
		{id: 2, norm: "う検索"},
		{id: 3, norm: "あ検索語"},
	}
	got := candidateIDs(rankSubstringPool("検索", pool))
	want := []int64{2, 3, 5, 4, 1}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("ranked IDs = %v, want %v", got, want)
	}
}

func candidateIDs(rows []candidateRow) []int64 {
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.id
	}
	return ids
}
