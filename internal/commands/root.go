package commands

import (
	"context"
	"fmt"

	"github.com/cirruslabs/chamber/internal/version"
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
		Use:   "chamber",
		Short: "Run commands in isolated Tart VMs - prevents prompt injection attacks and possible host destruction",
		Long: `Chamber runs commands inside ephemeral Tart virtual machines with the current directory mounted.
Similar to nohup, chamber clones a VM from the seed image, starts it with the working
directory mounted, executes the command inside the VM, and destroys the VM on exit.

🛡️  SECURITY: Perfect for AI agents in "YOLO" mode - prevents prompt injection attacks by
isolating execution in ephemeral VMs that are automatically destroyed after each run.

Example:
  chamber claude                                              # Run Claude AI in VM
  chamber claude --model opus .                               # Run Claude with specific model
  chamber codex                                               # Run Codex in VM
  chamber init ghcr.io/cirruslabs/macos-sequoia-base:latest   # Initialize chamber-seed VM
`,
		Version:       version.FullVersion,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no args or first arg is a known subcommand, show help
			if len(args) == 0 {
				return cmd.Help()
			}

			// Check if first arg is a subcommand
			for _, subCmd := range cmd.Commands() {
				if args[0] == subCmd.Name() {
					return fmt.Errorf("unknown command %q for %q", args[0], cmd.Name())
				}
			}

			// Backward compatibility: run command directly
			// Use interactive mode for better terminal support
			return runCommand(context.Background(), vmImage, cpuCount, memoryMB, sshUser, sshPass, true, args)
		},
	}

	// Add global flags for backward compatibility
	cmd.PersistentFlags().StringVar(&vmImage, "vm", "chamber-seed", "Tart VM image to use (default: chamber-seed)")
	cmd.PersistentFlags().Uint32Var(&cpuCount, "cpu", 0, "Number of CPUs (0 = default)")
	cmd.PersistentFlags().Uint32Var(&memoryMB, "memory", 0, "Memory in MB (0 = default)")
	cmd.PersistentFlags().StringVar(&sshUser, "ssh-user", "admin", "SSH username")
	cmd.PersistentFlags().StringVar(&sshPass, "ssh-pass", "admin", "SSH password")
	cmd.PersistentFlags().BoolVar(&dangerouslySkipPermissions, "dangerously-skip-permissions", false, "Skip permission checks (use with caution)")

	// Stop parsing flags after the first non-flag argument
	cmd.Flags().SetInterspersed(false)

	// Add subcommands
	cmd.AddCommand(NewInitCmd())
	cmd.AddCommand(NewClaudeCmd())
	cmd.AddCommand(NewCodexCmd())

	return cmd
}

func Execute() error {
	return NewRootCmd().Execute()
}
