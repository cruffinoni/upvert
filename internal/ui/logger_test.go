package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerColorizesWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, true).Infof("hello %s", "world")
	got := buf.String()
	if !strings.Contains(got, "\033[34m") || !strings.Contains(got, "hello world") {
		t.Fatalf("coloured output = %q", got)
	}
}

func TestLoggerWritesPlainTextWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, false).Successf("done")
	got := buf.String()
	if strings.Contains(got, "\033[") {
		t.Fatalf("plain output contains escape codes: %q", got)
	}
	if !strings.Contains(got, "[SUCCESS] done") {
		t.Fatalf("plain output = %q", got)
	}
}

func TestColorEnabledRejectsNonTerminal(t *testing.T) {
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Fatal("colours must be disabled for non-file writers")
	}
}
