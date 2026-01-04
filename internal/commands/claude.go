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

// parseDirectoryMounts parses --dir flag values into DirectoryMount structs.
// Format: name:path[:ro]
// Examples:
//   - data:~/my-data
//   - docs:/path/to/docs:ro
func parseDirectoryMounts(dirs []string) ([]tart.DirectoryMount, error) {
	var mounts []tart.DirectoryMount
	for _, dir := range dirs {
		parts := strings.Split(dir, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid --dir format: %q (expected name:path[:ro])", dir)
		}

		name := parts[0]
		path := parts[1]

		// Expand ~ to home directory
		if strings.HasPrefix(path, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home directory: %w", err)
			}
			path = filepath.Join(home, path[1:])
		}

		// Convert to absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for %q: %w", path, err)
		}

		// Check if read-only
		readOnly := false
		if len(parts) > 2 && parts[2] == "ro" {
			readOnly = true
		}

		mounts = append(mounts, tart.DirectoryMount{
			Name:     name,
			Path:     absPath,
			ReadOnly: readOnly,
		})
	}
	return mounts, nil
}

func NewClaudeCmd() *cobra.Command {
	var (
		vmImage string
	)

	cmd := &cobra.Command{
		Use:   "claude [flags] [claude-args...]",
		Short: "Run claude in an isolated Tart VM with --dangerously-skip-permissions",
		Long: `Run claude inside an ephemeral Tart virtual machine with the current directory mounted.
Automatically prepends --dangerously-skip-permissions to claude arguments for AI agent execution.

Example:
  chamber claude
  chamber claude --model=opus
  chamber claude --vm=macos-xcode`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Prepend claude command and --dangerously-skip-permissions flag
			claudeArgs := []string{"claude", "--dangerously-skip-permissions"}
			claudeArgs = append(claudeArgs, args...)
			return runCommand(cmd.Context(), vmImage, 0, 0, "admin", "admin", additionalDirs, true, claudeArgs)
		},
	}

	cmd.Flags().StringVar(&vmImage, "vm", "chamber-seed", "Tart VM image to use (default: chamber-seed)")

	// Stop parsing flags after the first non-flag argument AND disable flag parsing entirely for unknown flags
	cmd.Flags().SetInterspersed(false)
	cmd.DisableFlagParsing = false
	cmd.FParseErrWhitelist.UnknownFlags = true

	return cmd
}

func runCommand(ctx context.Context, vmImage string, cpuCount, memoryMB uint32, sshUser, sshPass string, extraDirs []string, interactive bool, args []string) error {
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

	// Extract directory name for dynamic mounting
	dirName := filepath.Base(cwd)

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
	fmt.Fprintf(os.Stdout, "Creating ephemeral VM from %s...\n", vmImage)
	vm, err := tart.NewVMClonedFrom(ctx, vmImage, nil)
	if err != nil {
		return err
	}
	defer func() {
		fmt.Fprintln(os.Stdout, "Cleaning up VM...")
		if err := vm.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clean up VM: %v\n", err)
		}
	}()

	// Configure VM
	fmt.Fprintln(os.Stdout, "Configuring VM...")
	if err := vm.Configure(ctx, cpuCount, memoryMB); err != nil {
		return err
	}

	// Start VM with directory mounts
	fmt.Fprintln(os.Stdout, "Starting VM...")
	directoryMounts := []tart.DirectoryMount{
		{
			Name:     dirName,
			Path:     cwd,
			ReadOnly: false,
		},
	}

	// Parse and add user-specified additional directories
	if len(extraDirs) > 0 {
		additionalMounts, err := parseDirectoryMounts(extraDirs)
		if err != nil {
			return err
		}
		directoryMounts = append(directoryMounts, additionalMounts...)
	}

	vm.Start(ctx, directoryMounts)

	// Wait for VM to get IP
	fmt.Fprintln(os.Stdout, "Waiting for VM to boot...")
	ip, err := vm.RetrieveIP(ctx)
	if err != nil {
		return fmt.Errorf("failed to get VM IP: %w", err)
	}
	fmt.Fprintf(os.Stdout, "VM IP: %s\n", ip)

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
	fmt.Fprintln(os.Stdout, "Connecting to VM via SSH...")
	sshAddr := fmt.Sprintf("%s:22", ip)
	sshClient, err := ssh.WaitForSSH(ctx, sshAddr, sshUser, sshPass)
	if err != nil {
		return fmt.Errorf("failed to connect via SSH: %w", err)
	}
	defer sshClient.Close()

	// Create executor
	exec := executor.New(sshClient, cwd, dirName)

	// Mount working directory
	fmt.Fprintln(os.Stdout, "Mounting working directory...")
	if err := exec.MountWorkingDirectory(ctx); err != nil {
		return err
	}
	defer func() {
		_ = exec.UnmountWorkingDirectory(ctx)
	}()

	// Execute command
	fmt.Fprintf(os.Stdout, "Executing command: %s %v\n", args[0], args[1:])
	fmt.Fprintln(os.Stdout, strings.Repeat("-", 80))

	// Use interactive or non-interactive execution based on the parameter
	if interactive {
		if err := exec.ExecuteInteractive(ctx, args[0], args[1:]); err != nil {
			return err
		}
	} else {
		if err := exec.Execute(ctx, args[0], args[1:]); err != nil {
			return err
		}
	}

	return nil
}
