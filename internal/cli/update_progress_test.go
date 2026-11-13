package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUpdateReporterWritesStepLines(t *testing.T) {
	var out bytes.Buffer
	reporter := newUpdateReporter(&out, false)

	reporter.Start("step")
	reporter.Finish("ok")

	assert.Contains(t, out.String(), "step")
	assert.Contains(t, out.String(), "ok")
}

func TestNewUpdateReporterQuiet(t *testing.T) {
	var out bytes.Buffer
	reporter := newUpdateReporter(&out, true)

	reporter.Start("step")
	reporter.Finish("ok")

	assert.Empty(t, out.String())
}
