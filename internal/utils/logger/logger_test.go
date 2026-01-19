package logger

import (
	"bytes"
	"strings"
	"testing"
)

// compile-time interface compliance check
var _ Logger = (*MyLogger)(nil)

func TestMyLogger_Basic(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, "", 0) // deterministic: no flags

	l.Info("hello", "world")
	l.Infof("value=%d", 42)
	l.Error("oops")
	l.Errorf("err=%s", "boom")
	l.Debug("dbg")
	l.Debugf("dbg=%s", "ok")

	out := buf.String()

	checks := []string{
		"[INFO] hello world",
		"[INFO] value=42",
		"[ERROR] oops",
		"[ERROR] err=boom",
		"[DEBUG] dbg",
		"[DEBUG] dbg=ok",
	}

	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Fatalf("expected %q in output, got: %q", c, out)
		}
	}
}

func TestNewWithNilWriterAndDefaults(t *testing.T) {
	// New with nil writer should return a usable logger (writes to stdout in that case).
	l := New(nil, "", 0)
	if l == nil {
		t.Fatal("expected non-nil logger from New(nil, ...)")
	}

	// Default logger should also be non-nil and satisfy the Logger interface.
	if Default() == nil {
		t.Fatal("Default() returned nil")
	}
	if DefaultLogger == nil {
		t.Fatal("DefaultLogger is nil")
	}
}
