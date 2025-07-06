package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cirruslabs/chamber/internal/commands"
)

func main() {
	// Set up signal interruptible context
	ctx, cancel := context.WithCancel(context.Background())

	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-interruptCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := mainImpl(ctx); err != nil {
		log.Fatal(err)
	}
}

func mainImpl(ctx context.Context) error { // Run the command
	return commands.NewRootCmd().ExecuteContext(ctx)
}
