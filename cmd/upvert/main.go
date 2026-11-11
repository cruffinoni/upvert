// Command upvert bumps semantic versions for Go, Node.js, Python, Rust, and
// multi-language repositories.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cruffinoni/upvert/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.New().ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "upvert: %v\n", err)
		os.Exit(1)
	}
}
