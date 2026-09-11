// Package dictionary defines the dictionary types handled by EJQuick.
package dictionary

import "fmt"

// Type identifies one of the bundled dictionary kinds.
type Type string

const (
	// Eiji is the English-Japanese dictionary (EIJIRO).
	Eiji Type = "eiji"
	// Waei is the Japanese-English dictionary (WAEIJI).
	Waei Type = "waei"
)

// All lists the known dictionary types in a stable order.
var All = []Type{Eiji, Waei}

// IsValid reports whether t is a known dictionary type.
func IsValid(t Type) bool {
	return t == Eiji || t == Waei
}

// ParseType converts a string (for example from CLI options or config
// files) into a Type, reporting unknown values as an error.
func ParseType(s string) (Type, error) {
	t := Type(s)
	if !IsValid(t) {
		return "", fmt.Errorf("unknown dictionary type %q (want %q or %q)", s, Eiji, Waei)
	}
	return t, nil
}

// Label returns the short status label shown in the TUI.
func (t Type) Label() string {
	switch t {
	case Eiji:
		return "EIJI"
	case Waei:
		return "WAEI"
	default:
		return string(t)
	}
}

// String implements fmt.Stringer.
func (t Type) String() string { return string(t) }

// Other returns the opposite dictionary type.
func (t Type) Other() Type {
	if t == Eiji {
		return Waei
	}
	return Eiji
}
