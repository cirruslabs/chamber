# Chamber

Chamber is a tool for running commands inside [Tart](https://github.com/cirruslabs/tart) virtual machines
with the current directory mounted. It provides a lightweight isolated environment for you agents in YOLO mode.

Don't think about prompt injection attacks anymore! Configure a macOS virtual machine only with YOLO-safe permissions
and run your agents inside it with Chamber.

## Features

- **VM Isolation Security**: Prevents prompt injection attacks by isolating AI agents in ephemeral VMs
- **Agent Safety**: Perfect for AI agents running with flags like `--dangerously-skip-permissions` or similar "YOLO" modes
- Run commands in isolated Tart VMs that are automatically destroyed after execution
- Automatic mounting of current directory
- SSH-based command execution with full stdio redirection
- Configurable CPU and memory allocation
- Automatic VM cleanup after execution (no persistence of malicious changes)

## Prerequisites

- macOS (Apple Silicon or Intel)
- [Tart](https://github.com/cirruslabs/tart) installed
- A Tart VM image (e.g., `ghcr.io/cirruslabs/macos-sequoia-xcode:latest`)

## Installation

```bash
go install github.com/cirruslabs/chamber/cmd/chamber@latest
```

Or build from source:

```bash
git clone https://github.com/cirruslabs/chamber.git
cd chamber
go build -o chamber ./cmd/chamber/
```

## Usage

Basic usage:

```bash
chamber --vm <tart-image> <command> [args...]
```

Examples:

```bash
# Run Swift tests in a VM
chamber --vm ghcr.io/cirruslabs/macos-ventura-base:latest swift test

# SECURE AI AGENT EXECUTION - Run claude with dangerous permissions in isolated VM
# This prevents prompt injection attacks by containing the agent in an ephemeral VM
chamber --vm macos-xcode claude --dangerously-skip-permissions

# Run any AI agent safely in "YOLO" mode
chamber --vm macos-xcode aider --yes --auto-commits

# Build a project with custom resources
chamber --vm my-custom-image --cpu 8 --memory 16384 make build

# Run a shell script
chamber --vm ghcr.io/cirruslabs/macos-ventura-base:latest ./build.sh
```

## Command Line Options

- `--vm` (required): Tart VM image to use
- `--cpu`: Number of CPUs to allocate (default: VM default)
- `--memory`: Memory in MB to allocate (default: VM default)
- `--ssh-user`: SSH username (default: "admin")
- `--ssh-pass`: SSH password (default: "admin")
- `--dangerously-skip-permissions`: Skip permission checks (use with caution - safe when used with Chamber!)

## 🛡️ Why Use Chamber for AI Agents?

**Problem**: AI agents running with permissive flags like `--dangerously-skip-permissions`, `--yes`, or `--auto-commits` are vulnerable to prompt injection attacks that can compromise your host system.

**Solution**: Chamber isolates AI agents in ephemeral VMs, making "YOLO" mode safe:

```bash
# ❌ DANGEROUS: Direct execution on host
claude --dangerously-skip-permissions

# ✅ SAFE: Isolated execution in ephemeral VM
chamber --vm=macos-xcode claude --dangerously-skip-permissions
```

**Key Benefits**:
- **Zero Host Risk**: Even if prompt injection succeeds, damage is contained in the VM
- **Automatic Cleanup**: VMs are destroyed after each run - no persistent compromise
- **Full Functionality**: AI agents work normally but can't escape the sandbox
- **Easy Integration**: Just prefix your existing AI agent commands with `chamber --vm=...`

## How It Works

1. Chamber clones a new **ephemeral** VM from the specified base image
2. Configures the VM with requested CPU/memory settings
3. Starts the VM with the current directory mounted via virtiofs
4. Establishes SSH connection to the VM
5. Mounts the working directory inside the VM at `$HOME/working-dir`
6. Executes the specified command in that directory (AI agents run in complete isolation)
7. Streams stdout/stderr back to the host (similar to nohup)
8. **Automatically destroys the VM** after execution (or on interrupt) - no persistence of malicious changes

## Security Benefits for AI Agents

Chamber provides critical security when running AI agents in "YOLO" mode:

- **Prompt Injection Protection**: Even if an AI agent is compromised via prompt injection, damage is contained within the ephemeral VM
- **No Host Contamination**: Malicious commands cannot affect the host system
- **Automatic Cleanup**: VMs are destroyed after each run, preventing persistence of attacks
- **Safe Experimentation**: Perfect for agents with `--dangerously-skip-permissions`, `--yes`, `--auto-commits` flags
- **Isolation Guarantee**: Each command runs in a fresh, isolated environment

## Architecture

Chamber is built on top of Tart's virtualization capabilities and uses:
- virtiofs for high-performance directory sharing
- SSH for secure command execution
- Go's context package for proper cleanup on interrupts

## License

This project is licensed under the MIT License - see the LICENSE file for details.