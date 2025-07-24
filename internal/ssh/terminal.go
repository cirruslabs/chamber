package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// Terminal provides SSH terminal proxying with full PTY support
type Terminal struct {
	client *ssh.Client
}

// NewTerminal creates a new SSH terminal proxy
func NewTerminal(client *ssh.Client) *Terminal {
	return &Terminal{client: client}
}

// RunInteractiveCommand runs a command with full terminal proxying
func (t *Terminal) RunInteractiveCommand(ctx context.Context, command string) error {
	session, err := t.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Check if we're running in a terminal
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Fallback to non-interactive mode
		return t.runNonInteractive(session, command)
	}

	// Save terminal state
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("failed to make terminal raw: %w", err)
	}

	// Ensure terminal is restored
	restored := false
	restore := func() {
		if !restored {
			_ = term.Restore(fd, oldState)
			restored = true
		}
	}
	defer restore()

	// Handle panics
	defer func() {
		if r := recover(); r != nil {
			restore()
			panic(r)
		}
	}()

	// Get terminal size
	width, height, err := term.GetSize(fd)
	if err != nil {
		return fmt.Errorf("failed to get terminal size: %w", err)
	}

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("xterm-256color", height, width, modes); err != nil {
		return fmt.Errorf("failed to request pty: %w", err)
	}

	// Set up pipes
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Handle window resize
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go t.handleWindowResize(ctx, session, fd)

	// Start command
	if err := session.Start(command); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Copy IO
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(stdin, os.Stdin)
		_ = stdin.Close()
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stdout, stdout)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stderr, stderr)
	}()

	// Wait for command to complete
	err = session.Wait()
	cancel() // Stop resize handler
	wg.Wait()

	restore()

	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// handleWindowResize handles terminal window resize events
func (t *Terminal) handleWindowResize(ctx context.Context, session *ssh.Session, fd int) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	defer signal.Stop(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			width, height, _ := term.GetSize(fd)
			if session != nil {
				_ = session.WindowChange(height, width)
			}
		}
	}
}

// runNonInteractive runs command without PTY for non-terminal environments
func (t *Terminal) runNonInteractive(session *ssh.Session, command string) error {
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	return session.Run(command)
}
