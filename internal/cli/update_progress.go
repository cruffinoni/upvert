package cli

import (
	"io"

	"github.com/cruffinoni/upvert/internal/ui"
	"github.com/cruffinoni/upvert/internal/update"
)

// newUpdateReporter selects quiet, animated terminal, or plain line output.
func newUpdateReporter(out io.Writer, quiet bool) update.Reporter {
	return ui.NewProgress(out, ui.ProgressOptions{Quiet: quiet})
}
