package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cirruslabs/chamber/internal/executor"
	"github.com/cirruslabs/chamber/internal/ssh"
	"github.com/cirruslabs/chamber/internal/vm/tart"
	"github.com/spf13/cobra"
)

func NewClaudeCmd() *cobra.Command {
	var (
		vmImage  string
		cpuCount uint32
		memoryMB uint32
		sshUser  string
		sshPass  string
	)

	cmd := &cobra.Command{
		Use:   "claude [flags] [claude-args...]",
		Short: "Run claude in an isolated Tart VM with --dangerously-skip-permissions",
		Long: `Run claude inside an ephemeral Tart virtual machine with the current directory mounted.
Automatically prepends --dangerously-skip-permissions to claude arguments for AI agent execution.

Example:
  chamber claude
  chamber claude --model opus-3.5
  chamber claude --vm=macos-xcode --continue`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Prepend claude command and --dangerously-skip-permissions flag
			claudeArgs := []string{"claude", "--dangerously-skip-permissions"}
			claudeArgs = append(claudeArgs, args...)
			return runCommand(cmd.Context(), vmImage, cpuCount, memoryMB, sshUser, sshPass, true, claudeArgs)
		},
	}

	cmd.Flags().StringVar(&vmImage, "vm", "chamber-seed", "Tart VM image to use (default: chamber-seed)")
	cmd.Flags().Uint32Var(&cpuCount, "cpu", 0, "Number of CPUs (0 = default)")
	cmd.Flags().Uint32Var(&memoryMB, "memory", 0, "Memory in MB (0 = default)")
	cmd.Flags().StringVar(&sshUser, "ssh-user", "admin", "SSH username")
	cmd.Flags().StringVar(&sshPass, "ssh-pass", "admin", "SSH password")

	// Stop parsing flags after the first non-flag argument
	cmd.Flags().SetInterspersed(false)

	return cmd
}

func runCommand(ctx context.Context, vmImage string, cpuCount, memoryMB uint32, sshUser, sshPass string, dangerouslySkipPermissions bool, args []string) error {
	// Check if Tart is installed
	if !tart.Installed() {
		return fmt.Errorf("tart is not installed. Please install it from https://github.com/cirruslabs/tart")
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create context with cancellation
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

	// Create VM
	fmt.Fprintf(os.Stderr, "Creating ephemeral VM from %s...\n", vmImage)
	vm, err := tart.NewVMClonedFrom(ctx, vmImage, nil)
	if err != nil {
		return err
	}
	defer func() {
		fmt.Fprintln(os.Stderr, "Cleaning up VM...")
		if err := vm.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clean up VM: %v\n", err)
		}
	}()

	// Configure VM
	fmt.Fprintln(os.Stderr, "Configuring VM...")
	if err := vm.Configure(ctx, cpuCount, memoryMB); err != nil {
		return err
	}

	// Start VM with directory mount
	fmt.Fprintln(os.Stderr, "Starting VM...")
	directoryMounts := []tart.DirectoryMount{
		{
			Name:     "workspace",
			Path:     cwd,
			Tag:      "tart.virtiofs.workspace",
			ReadOnly: false,
		},
	}
	vm.Start(ctx, directoryMounts)

	// Wait for VM to get IP
	fmt.Fprintln(os.Stderr, "Waiting for VM to boot...")
	ip, err := vm.RetrieveIP(ctx)
	if err != nil {
		return fmt.Errorf("failed to get VM IP: %w", err)
	}
	fmt.Fprintf(os.Stderr, "VM IP: %s\n", ip)

	// Check for VM startup errors
	select {
	case err := <-vm.ErrChan():
		if err != nil {
			return fmt.Errorf("VM failed to start: %w", err)
		}
	default:
		// VM is running
	}

	// Connect via SSH
	fmt.Fprintln(os.Stderr, "Connecting to VM via SSH...")
	sshAddr := fmt.Sprintf("%s:22", ip)
	sshClient, err := ssh.WaitForSSH(ctx, sshAddr, sshUser, sshPass)
	if err != nil {
		return fmt.Errorf("failed to connect via SSH: %w", err)
	}
	defer sshClient.Close()

	// Create executor
	exec := executor.New(sshClient, cwd)

	// Mount working directory
	fmt.Fprintln(os.Stderr, "Mounting working directory...")
	if err := exec.MountWorkingDirectory(ctx); err != nil {
		return err
	}
	defer exec.UnmountWorkingDirectory(ctx)

	// Execute command
	fmt.Fprintf(os.Stderr, "Executing command: %s %v\n", args[0], args[1:])
	fmt.Fprintln(os.Stderr, strings.Repeat("-", 80))

	if err := exec.Execute(ctx, args[0], args[1:]); err != nil {
		return err
	}

	return nil
}