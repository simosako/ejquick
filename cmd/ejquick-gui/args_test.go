package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	var stdout bytes.Buffer
	opts, done, err := parseArgs([]string{"--config", "custom.toml", "--debug"}, &stdout)
	if err != nil || done {
		t.Fatalf("parseArgs = %#v, %t, %v", opts, done, err)
	}
	if !opts.configPathSet || opts.configPath != "custom.toml" || !opts.debug {
		t.Errorf("options = %#v", opts)
	}
}

func TestParseArgsHelpAndVersion(t *testing.T) {
	for _, arg := range []string{"--help", "--version"} {
		t.Run(arg, func(t *testing.T) {
			var stdout bytes.Buffer
			_, done, err := parseArgs([]string{arg}, &stdout)
			if err != nil || !done || stdout.Len() == 0 {
				t.Fatalf("parseArgs(%q): done=%t err=%v stdout=%q", arg, done, err, stdout.String())
			}
		})
	}
}

func TestParseArgsRejectsUnsupportedArguments(t *testing.T) {
	tests := [][]string{
		{"query"},
		{"--dictionary", "eiwa"},
		{"--unknown"},
		{"--config"},
	}
	for _, args := range tests {
		if _, _, err := parseArgs(args, &bytes.Buffer{}); err == nil {
			t.Errorf("parseArgs(%q) unexpectedly succeeded", strings.Join(args, " "))
		}
	}
}
