package main

import (
	"bytes"
	"testing"
)

// T-036 (2.4): version is a var the release build stamps with -X main.version.
func TestPrintVersion(t *testing.T) {
	if version != "0.1.0-dev" {
		t.Fatalf("version = %q, want the 0.1.0-dev default", version)
	}
	var out bytes.Buffer
	printVersion(&out)
	if out.String() != "waggle v0.1.0-dev\n" {
		t.Errorf("printVersion = %q", out.String())
	}
}
