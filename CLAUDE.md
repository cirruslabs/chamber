# Claude Development Instructions

## Code Quality Checks

Before completing any task, always run the following commands to ensure code quality:

1. **Run the linter**: `golangci-lint run -v`
   - Fix any issues reported by the linter
   - The project uses a custom `.golangci.yml` configuration

2. **Build the project**: `go build ./...`
   - Ensure all packages compile without errors

## Project Structure

This is a Go project for the Chamber tool, which manages VMs for development environments.