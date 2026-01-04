package commands

import (
	"github.com/spf13/cobra"
)

func NewCodexCmd() *cobra.Command {
	var vmImage string

	cmd := &cobra.Command{
		Use:   "codex [flags] [codex-args...]",
		Short: "Run codex in an isolated Tart VM with --dangerously-bypass-approvals-and-sandbox",
		Long: `Run codex inside an ephemeral Tart virtual machine with the current directory mounted.
Automatically prepends --dangerously-bypass-approvals-and-sandbox to codex arguments for AI agent execution.

Example:
  chamber codex
  chamber codex --vm=macos-xcode`,
		RunE: func(cmd *cobra.Command, args []string) error {
			codexArgs := []string{"codex", "--dangerously-bypass-approvals-and-sandbox"}
			codexArgs = append(codexArgs, args...)
			return runCommand(cmd.Context(), vmImage, 0, 0, "admin", "admin", additionalDirs, true, codexArgs)
		},
	}

	cmd.Flags().StringVar(&vmImage, "vm", "chamber-seed", "Tart VM image to use (default: chamber-seed)")

	// Stop parsing flags after the first non-flag argument AND disable flag parsing entirely for unknown flags
	cmd.Flags().SetInterspersed(false)
	cmd.DisableFlagParsing = false
	cmd.FParseErrWhitelist.UnknownFlags = true

	return cmd
}
