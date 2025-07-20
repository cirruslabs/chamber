package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

type Executor struct {
	sshClient      *ssh.Client
	workingDir     string
	mountedWorkDir string
}

func New(sshClient *ssh.Client, workingDir string) *Executor {
	return &Executor{
		sshClient:      sshClient,
		workingDir:     workingDir,
		mountedWorkDir: "$HOME/workspace",
	}
}

func (e *Executor) MountWorkingDirectory(ctx context.Context) error {
	session, err := e.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Create mount point and mount virtiofs
	tag := "tart.virtiofs.workspace"
	command := fmt.Sprintf("mkdir -p %q && mount_virtiofs %q %q", e.mountedWorkDir, tag, e.mountedWorkDir)

	if err := session.Run(command); err != nil {
		return fmt.Errorf("failed to mount working directory: %w", err)
	}

	return nil
}

func (e *Executor) UnmountWorkingDirectory(ctx context.Context) error {
	session, err := e.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	command := fmt.Sprintf("umount %q", e.mountedWorkDir)

	// Ignore errors on unmount as it might have been unmounted already
	_ = session.Run(command)

	return nil
}

func (e *Executor) Execute(ctx context.Context, command string, args []string) error {
	session, err := e.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Set up pipes for stdout and stderr
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Start output readers
	go e.streamOutput(stdout, os.Stdout)
	go e.streamOutput(stderr, os.Stderr)

	// Start a shell
	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	// Change to mounted working directory
	_, err = stdin.Write([]byte(fmt.Sprintf("cd %s\n", e.mountedWorkDir)))
	if err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	// Execute the command
	fullCommand := fmt.Sprintf("%s %s", command, strings.Join(args, " "))
	_, err = stdin.Write([]byte(fullCommand + "\nexit $?\n"))
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	// Wait for command to complete
	if err := session.Wait(); err != nil {
		// Check if it's an exit error, which means the command ran but returned non-zero
		if exitErr, ok := err.(*ssh.ExitError); ok {
			// Return a more descriptive error
			return fmt.Errorf("command exited with status %d", exitErr.ExitStatus())
		}
		return fmt.Errorf("failed to run command: %w", err)
	}

	return nil
}

func (e *Executor) streamOutput(reader io.Reader, writer io.Writer) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fmt.Fprintln(writer, scanner.Text())
	}
}
