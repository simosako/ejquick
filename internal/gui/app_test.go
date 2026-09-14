//go:build gui

package gui

import (
	"os"
	"testing"
)

func TestSkipPortalServicesSetsFlagWhenUnset(t *testing.T) {
	t.Setenv("QT_NO_XDG_DESKTOP_PORTAL", "")
	skipPortalServices()
	if got := os.Getenv("QT_NO_XDG_DESKTOP_PORTAL"); got != "1" {
		t.Errorf("QT_NO_XDG_DESKTOP_PORTAL = %q, want %q", got, "1")
	}
}

func TestSkipPortalServicesKeepsExplicitValue(t *testing.T) {
	t.Setenv("QT_NO_XDG_DESKTOP_PORTAL", "0")
	skipPortalServices()
	if got := os.Getenv("QT_NO_XDG_DESKTOP_PORTAL"); got != "0" {
		t.Errorf("QT_NO_XDG_DESKTOP_PORTAL = %q, want %q", got, "0")
	}
}
