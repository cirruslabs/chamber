# Chamber

Chamber is a tool for running commands inside Tart virtual machines with the current directory mounted via virtiofs. It provides an isolated environment for building and testing macOS applications.

## Features

- Run commands in isolated Tart VMs
- Automatic mounting of current directory via virtiofs
- SSH-based command execution
- Configurable CPU and memory allocation
- Automatic VM cleanup after execution

## Prerequisites

- macOS (Apple Silicon or Intel)
- [Tart](https://github.com/cirruslabs/tart) installed
- A Tart VM image (e.g., `ghcr.io/cirruslabs/macos-ventura-base:latest`)

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
chamber --image <tart-image> -- <command> [args...]
```

Examples:

```bash
# Run Swift tests in a VM
chamber --image ghcr.io/cirruslabs/macos-ventura-base:latest -- swift test

# Build a project with custom resources
chamber --image my-custom-image --cpu 8 --memory 16384 -- make build

# Run a shell script
chamber --image ghcr.io/cirruslabs/macos-ventura-base:latest -- ./build.sh
```

## Command Line Options

- `--image` (required): Tart VM image to use
- `--cpu`: Number of CPUs to allocate (default: VM default)
- `--memory`: Memory in MB to allocate (default: VM default)
- `--ssh-user`: SSH username (default: "admin")
- `--ssh-pass`: SSH password (default: "admin")

## How It Works

1. Chamber clones a new VM from the specified base image
2. Configures the VM with requested CPU/memory settings
3. Starts the VM with the current directory mounted via virtiofs
4. Establishes SSH connection to the VM
5. Mounts the working directory inside the VM at `$HOME/working-dir`
6. Executes the specified command in that directory
7. Streams stdout/stderr back to the host
8. Cleans up the VM after execution (or on interrupt)

## Architecture

Chamber is built on top of Tart's virtualization capabilities and uses:
- virtiofs for high-performance directory sharing
- SSH for secure command execution
- Go's context package for proper cleanup on interrupts

## License

This project is licensed under the MIT License - see the LICENSE file for details.