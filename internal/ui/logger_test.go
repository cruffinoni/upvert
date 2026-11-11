package ui

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerColorizesWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, true).Infof("hello %s", "world")
	got := buf.String()
	assert.Contains(t, got, "\033[34m")
	assert.Contains(t, got, "hello world")
}

func TestLoggerWritesPlainTextWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, false).Successf("done")
	got := buf.String()
	assert.NotContains(t, got, "\033[")
	assert.Contains(t, got, "[SUCCESS] done")
}

func TestColorEnabledRejectsNonTerminal(t *testing.T) {
	var buf bytes.Buffer
	assert.False(t, ColorEnabled(&buf))
}
