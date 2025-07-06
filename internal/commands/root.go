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
	"github.com/cirruslabs/chamber/internal/version"
	"github.com/cirruslabs/chamber/internal/vm/tart"
	"github.com/spf13/cobra"
)

var (
	vmImage                    string
	cpuCount                   uint32
	memoryMB                   uint32
	sshUser                    string
	sshPass                    string
	dangerouslySkipPermissions bool
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chamber [flags] command [args...]",
		Short: "Run commands in Tart VMs",
		Long: `Chamber runs commands inside Tart virtual machines with the current directory mounted.
Similar to nohup, chamber clones a VM from the specified image, starts it with the working
directory mounted, executes the command inside the VM, and destroys the VM on exit.

Example:
  chamber --vm=macos-ventura-base swift test
  chamber --vm=macos-xcode claude --dangerously-skip-permissions
  chamber --vm=macos-xcode make build`,
		Version:       version.FullVersion,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(1),
		RunE:          run,
	}

	cmd.Flags().StringVar(&vmImage, "vm", "", "Tart VM image to use (required)")
	cmd.Flags().Uint32Var(&cpuCount, "cpu", 0, "Number of CPUs (0 = default)")
	cmd.Flags().Uint32Var(&memoryMB, "memory", 0, "Memory in MB (0 = default)")
	cmd.Flags().StringVar(&sshUser, "ssh-user", "admin", "SSH username")
	cmd.Flags().StringVar(&sshPass, "ssh-pass", "admin", "SSH password")
	cmd.Flags().BoolVar(&dangerouslySkipPermissions, "dangerously-skip-permissions", false, "Skip permission checks (use with caution)")
	cmd.MarkFlagRequired("vm")

	return cmd
}

func run(cmd *cobra.Command, args []string) error {
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
	ctx, cancel := context.WithCancel(context.Background())
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
			Name:     "working-dir",
			Path:     cwd,
			Tag:      "tart.virtiofs.working-dir",
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
	exec := executor.New(sshClient, cwd, os.Environ())

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
