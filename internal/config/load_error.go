package config

import "errors"

// LoadCategory classifies failures returned by Load without exposing parser or
// operating-system details to frontends.
type LoadCategory string

const (
	LoadUnreadable LoadCategory = "unreadable"
	LoadInvalid    LoadCategory = "invalid"
)

// LoadError preserves a configuration load failure and its stable category.
type LoadError struct {
	Category LoadCategory
	Err      error
}

func (e *LoadError) Error() string { return e.Err.Error() }
func (e *LoadError) Unwrap() error { return e.Err }

// LoadCategoryOf returns the category carried by err and whether it was
// produced by Load.
func LoadCategoryOf(err error) (LoadCategory, bool) {
	var loadErr *LoadError
	if !errors.As(err, &loadErr) {
		return "", false
	}
	return loadErr.Category, true
}

func loadFailure(category LoadCategory, err error) error {
	return &LoadError{Category: category, Err: err}
}
