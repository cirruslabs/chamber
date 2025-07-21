package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cirruslabs/chamber/internal/ssh"
	"github.com/cirruslabs/chamber/internal/vm/tart"
	"github.com/spf13/cobra"
)

func NewInitCmd() *cobra.Command {
	var remoteVM string

	cmd := &cobra.Command{
		Use:   "init <remote-vm>",
		Short: "Initialize chamber by cloning a remote VM and setting up Claude Code",
		Long: `Initialize chamber by:
1. Cloning a remote Tart VM to 'chamber-seed' local VM
2. Installing @anthropic-ai/claude-code globally via npm
3. Running claude setup-token with output redirected to current terminal

Example:
  chamber init ghcr.io/cirruslabs/macos-sequoia-base:latest`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			remoteVM = args[0]
			return runInit(cmd.Context(), remoteVM)
		},
	}

	return cmd
}

func runInit(ctx context.Context, remoteVM string) error {
	// Check if Tart is installed
	if !tart.Installed() {
		return fmt.Errorf("tart is not installed. Please install it from https://github.com/cirruslabs/tart")
	}

	// Create context with cancellation if not provided
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle interrupts
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nInterrupted, cleaning up...")
		cancel()
	}()

	// Clone the remote VM to chamber-seed
	fmt.Fprintf(os.Stdout, "Cloning %s to chamber-seed...\n", remoteVM)
	if err := tart.CloneVM(ctx, remoteVM, "chamber-seed"); err != nil {
		return fmt.Errorf("failed to clone VM: %w", err)
	}

	// Start the chamber-seed VM
	fmt.Fprintln(os.Stdout, "Starting chamber-seed VM...")
	vm, err := tart.NewVM(ctx, "chamber-seed")
	if err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}
	defer func() {
		fmt.Fprintln(os.Stdout, "Stopping VM...")
		if err := vm.StopWithContext(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to stop VM: %v\n", err)
		}
	}()

	// Start VM without directory mounts
	vm.Start(ctx, nil)

	// Wait for VM to get IP
	fmt.Fprintln(os.Stdout, "Waiting for VM to boot...")
	ip, err := vm.RetrieveIP(ctx)
	if err != nil {
		return fmt.Errorf("failed to get VM IP: %w", err)
	}
	fmt.Fprintf(os.Stdout, "VM IP: %s\n", ip)

	// Connect via SSH
	fmt.Fprintln(os.Stdout, "Connecting to VM via SSH...")
	sshAddr := fmt.Sprintf("%s:22", ip)
	sshClient, err := ssh.WaitForSSH(ctx, sshAddr, "admin", "admin")
	if err != nil {
		return fmt.Errorf("failed to connect via SSH: %w", err)
	}
	defer sshClient.Close()

	// Install Claude Code
	fmt.Fprintln(os.Stdout, "Installing @anthropic-ai/claude-code...")
	session, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	if err := session.Run("npm install -g @anthropic-ai/claude-code"); err != nil {
		return fmt.Errorf("failed to install claude-code: %w", err)
	}

	// Run claude setup-token with terminal attached
	fmt.Fprintln(os.Stdout, "\nSetting up Claude token...")
	fmt.Fprintln(os.Stdout, "Please follow the instructions below:\n")

	session2, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session2.Close()

	// Attach stdin/stdout/stderr for interactive setup
	session2.Stdin = os.Stdin
	session2.Stdout = os.Stdout
	session2.Stderr = os.Stderr

	// Request a pseudo-terminal for interactive mode
	if err := session2.RequestPty("xterm", 80, 24, nil); err != nil {
		return fmt.Errorf("failed to request PTY: %w", err)
	}

	if err := session2.Run("claude setup-token"); err != nil {
		return fmt.Errorf("failed to run claude setup-token: %w", err)
	}

	fmt.Fprintln(os.Stdout, "\nInitialization complete! chamber-seed VM is ready to use.")
	return nil
}
