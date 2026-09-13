package startup

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

type fakeService struct {
	closeErr error
	closed   int
}

func (*fakeService) Search(context.Context, string) ([]search.Entry, error) { return nil, nil }
func (s *fakeService) Close() error {
	s.closed++
	return s.closeErr
}

func startupConfig(defaultDict dictionary.Type) *config.Config {
	return &config.Config{
		Eiwa: config.Dictionary{Database: "eiwa.sqlite3"},
		Waei: config.Dictionary{Database: "waei.sqlite3"},
		Search: config.Search{
			DefaultDictionary: defaultDict.String(),
			MaxResults:        50,
		},
	}
}

func TestOpenDatabasesChecksDefaultFirstAndFallsBack(t *testing.T) {
	eiwa := &fakeService{}
	var order []dictionary.Type
	state := openDatabases(startupConfig(dictionary.Waei), nil,
		func(path string, dict dictionary.Type, maxResults int) (Service, error) {
			order = append(order, dict)
			if path != dict.String()+".sqlite3" || maxResults != 50 {
				t.Errorf("open(%q, %q, %d)", path, dict, maxResults)
			}
			if dict == dictionary.Waei {
				return nil, &search.OpenError{Category: search.OpenUnreadable, Err: errors.New("private detail")}
			}
			return eiwa, nil
		})

	if fmt.Sprint(order) != fmt.Sprint([]dictionary.Type{dictionary.Waei, dictionary.Eiwa}) {
		t.Errorf("open order = %v", order)
	}
	if state.Initial != dictionary.Eiwa {
		t.Errorf("initial = %q, want %q", state.Initial, dictionary.Eiwa)
	}
	if got := state.Statuses[dictionary.Waei]; got.Available || got.Category != search.OpenUnreadable || got.Err == nil {
		t.Errorf("waei status = %#v", got)
	}
	if got := state.Statuses[dictionary.Eiwa]; !got.Available || got.Category != "" || got.Err != nil {
		t.Errorf("eiwa status = %#v", got)
	}
	if state.Services[dictionary.Eiwa] != eiwa || state.Services[dictionary.Waei] != nil {
		t.Errorf("services = %#v", state.Services)
	}
}

func TestOpenDatabasesKeepsDefaultWhenBothUnavailable(t *testing.T) {
	calls := 0
	state := openDatabases(startupConfig(dictionary.Waei), nil,
		func(string, dictionary.Type, int) (Service, error) {
			calls++
			return nil, errors.New("unknown")
		})
	if calls != 2 {
		t.Errorf("open calls = %d, want 2", calls)
	}
	if state.Initial != dictionary.Waei {
		t.Errorf("initial = %q, want configured default", state.Initial)
	}
	for _, dict := range dictionary.All {
		if got := state.Statuses[dict]; got.Available || got.Category != search.OpenUnknown {
			t.Errorf("%s status = %#v", dict, got)
		}
	}
}

func TestDatabasesCloseIsIdempotentAndJoinsErrors(t *testing.T) {
	eiwa := &fakeService{closeErr: errors.New("eiwa close")}
	waei := &fakeService{closeErr: errors.New("waei close")}
	state := &Databases{Services: map[dictionary.Type]Service{
		dictionary.Eiwa: eiwa,
		dictionary.Waei: waei,
	}}
	err := state.Close()
	if err == nil || !errors.Is(err, eiwa.closeErr) || !errors.Is(err, waei.closeErr) {
		t.Fatalf("Close error = %v", err)
	}
	if err := state.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if eiwa.closed != 1 || waei.closed != 1 {
		t.Errorf("close counts = %d, %d", eiwa.closed, waei.closed)
	}
}

func TestOpenDatabasesClosesServiceReturnedWithError(t *testing.T) {
	unexpected := &fakeService{}
	state := openDatabases(startupConfig(dictionary.Eiwa), nil,
		func(string, dictionary.Type, int) (Service, error) {
			return unexpected, errors.New("open failed")
		})
	if unexpected.closed != 2 {
		t.Errorf("unexpected services closed = %d, want 2", unexpected.closed)
	}
	if len(state.Services) != 0 {
		t.Errorf("services = %#v, want empty", state.Services)
	}
}
