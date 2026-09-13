package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

type searchFunc func(context.Context, string) ([]search.Entry, error)

func (f searchFunc) Search(ctx context.Context, query string) ([]search.Entry, error) {
	return f(ctx, query)
}

func TestEmptyQueryDoesNotSearch(t *testing.T) {
	called := false
	controller := New(dictionary.Eiwa, map[dictionary.Type]Searcher{
		dictionary.Eiwa: searchFunc(func(context.Context, string) ([]search.Entry, error) {
			called = true
			return nil, nil
		}),
	}, 50, nil)

	if !controller.SetQuery("   ") {
		t.Fatal("normalization-empty query was not accepted")
	}
	if called {
		t.Fatal("normalization-empty query called the search service")
	}
	if got := controller.Snapshot(); got.Content != ContentEmpty || len(got.Entries) != 0 {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestNewQueryCancelsOldSearchAndDropsStaleEvent(t *testing.T) {
	events := make(chan Event, 2)
	firstStarted := make(chan struct{})
	service := searchFunc(func(ctx context.Context, query string) ([]search.Entry, error) {
		if query == "a" {
			close(firstStarted)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []search.Entry{{ID: 2, Headword: "ab", Body: "new"}}, nil
	})
	controller := New(dictionary.Eiwa, map[dictionary.Type]Searcher{dictionary.Eiwa: service}, 50,
		DispatchFunc(func(event Event) bool {
			events <- event
			return true
		}))

	controller.SetQuery("a")
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first search did not start")
	}
	controller.SetQuery("ab")
	for range 2 {
		select {
		case event := <-events:
			controller.Apply(event)
		case <-time.After(time.Second):
			t.Fatal("search event timeout")
		}
	}
	got := controller.Snapshot()
	if got.Query != "ab" || got.Content != ContentResults || len(got.Entries) != 1 || got.Entries[0].ID != 2 {
		t.Fatalf("snapshot = %+v", got)
	}
	controller.BeginClose()
	controller.Wait()
}

func TestApplyPreservesSelectionByEntryID(t *testing.T) {
	controller := New(dictionary.Eiwa, map[dictionary.Type]Searcher{}, 50, nil)
	controller.query = "a"
	controller.requestID = 7
	controller.entries = []search.Entry{{ID: 1}, {ID: 4}}
	controller.selected = 1
	controller.content = ContentResults

	if !controller.Apply(Event{
		RequestID: 7, Dictionary: dictionary.Eiwa,
		Entries: []search.Entry{{ID: 3}, {ID: 4}, {ID: 5}},
	}) {
		t.Fatal("current event was not applied")
	}
	if got := controller.Snapshot().Selected; got != 1 {
		t.Fatalf("selected row = %d, want 1", got)
	}
	if controller.Apply(Event{RequestID: 6, Dictionary: dictionary.Eiwa, Err: errors.New("stale")}) {
		t.Fatal("stale event was applied")
	}
}

func TestSwitchDictionaryClearsAllSearchState(t *testing.T) {
	services := map[dictionary.Type]Searcher{
		dictionary.Eiwa: searchFunc(func(context.Context, string) ([]search.Entry, error) { return nil, nil }),
		dictionary.Waei: searchFunc(func(context.Context, string) ([]search.Entry, error) { return nil, nil }),
	}
	controller := New(dictionary.Eiwa, services, 50, nil)
	controller.query = "old"
	controller.entries = []search.Entry{{ID: 1}}
	controller.selected = 0
	controller.content = ContentResults

	if !controller.SwitchDictionary(dictionary.Waei) {
		t.Fatal("switch failed")
	}
	got := controller.Snapshot()
	if got.Dictionary != dictionary.Waei || got.Query != "" || got.Selected != -1 || got.Content != ContentEmpty || len(got.Entries) != 0 {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestSearchErrorUsesGenericState(t *testing.T) {
	controller := New(dictionary.Eiwa, nil, 50, nil)
	controller.query = "query"
	controller.requestID = 1
	if !controller.Apply(Event{
		RequestID: 1, Dictionary: dictionary.Eiwa,
		Err: errors.New("private /path and SQL detail"),
	}) {
		t.Fatal("error event was not applied")
	}
	got := controller.Snapshot()
	if got.Content != ContentSearchError || len(got.Entries) != 0 || got.Query != "query" {
		t.Fatalf("snapshot = %+v", got)
	}
}
