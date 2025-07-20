package tart

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type VM struct {
	ident              string
	baseImage          string
	env                map[string]string
	runningVMCtx       context.Context
	runningVMCtxCancel context.CancelFunc
	wg                 sync.WaitGroup
	errChan            chan error
}

type DirectoryMount struct {
	Name     string
	Path     string
	Tag      string
	ReadOnly bool
}

const (
	vmNamePrefix = "chamber-ephemeral-"
)

func NewVMClonedFrom(
	ctx context.Context,
	from string,
	env map[string]string,
) (*VM, error) {
	runningVMCtx, runningVMCtxCancel := context.WithCancel(context.Background())

	// Use timestamp for VM name
	timestamp := time.Now().Format("20060102-150405")
	tmpVMName := vmNamePrefix + timestamp

	vm := &VM{
		ident:              tmpVMName,
		baseImage:          from,
		env:                env,
		runningVMCtx:       runningVMCtx,
		runningVMCtxCancel: runningVMCtxCancel,
		errChan:            make(chan error, 1),
	}

	// Clone the VM
	if err := Cmd(ctx, vm.env, "clone", from, vm.ident); err != nil {
		return nil, fmt.Errorf("failed to clone VM from %q: %w", from, err)
	}

	return vm, nil
}

func (vm *VM) Ident() string {
	return vm.ident
}

func (vm *VM) Configure(ctx context.Context, cpu uint32, memory uint32) error {
	// Set random MAC address to avoid conflicts
	if err := Cmd(ctx, vm.env, "set", vm.ident, "--random-mac"); err != nil {
		return fmt.Errorf("failed to set random MAC: %w", err)
	}

	if cpu != 0 {
		cpuStr := fmt.Sprintf("%d", cpu)
		if err := Cmd(ctx, vm.env, "set", vm.ident, "--cpu", cpuStr); err != nil {
			return fmt.Errorf("failed to set CPU count: %w", err)
		}
	}

	if memory != 0 {
		memoryStr := fmt.Sprintf("%d", memory)
		if err := Cmd(ctx, vm.env, "set", vm.ident, "--memory", memoryStr); err != nil {
			return fmt.Errorf("failed to set memory: %w", err)
		}
	}

	return nil
}

func (vm *VM) Start(ctx context.Context, directoryMounts []DirectoryMount) {
	vm.wg.Add(1)

	go func() {
		defer vm.wg.Done()

		args := []string{"--no-graphics", "--no-clipboard", "--no-audio"}

		for _, dm := range directoryMounts {
			var opts []string

			if tag := dm.Tag; tag != "" {
				opts = append(opts, fmt.Sprintf("tag=%s", tag))
			}

			if dm.ReadOnly {
				opts = append(opts, "ro")
			}

			dirArgumentValue := fmt.Sprintf("%s:%s", dm.Name, dm.Path)

			if len(opts) != 0 {
				dirArgumentValue += ":" + strings.Join(opts, ",")
			}

			args = append(args, "--dir", dirArgumentValue)
		}

		args = append(args, vm.ident)

		err := Cmd(vm.runningVMCtx, vm.env, "run", args...)
		vm.errChan <- err
	}()
}

func (vm *VM) ErrChan() chan error {
	return vm.errChan
}

func (vm *VM) RetrieveIP(ctx context.Context) (string, error) {
	// Wait up to 30 seconds for the VM to get an IP
	stdout, _, err := CmdWithCapture(ctx, vm.env, "ip", "--wait", "30", vm.ident)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(stdout), nil
}

func (vm *VM) Stop() error {
	return vm.StopWithContext(context.Background())
}

func (vm *VM) StopWithContext(ctx context.Context) error {
	// Try to gracefully stop the VM
	_ = Cmd(ctx, vm.env, "stop", "--timeout", "5", vm.ident)

	vm.runningVMCtxCancel()
	vm.wg.Wait()

	return nil
}

func (vm *VM) Delete() error {
	ctx := context.Background()
	err := Cmd(ctx, vm.env, "delete", vm.ident)
	return err
}

func (vm *VM) Close() error {
	if err := vm.Stop(); err != nil {
		return err
	}
	return vm.Delete()
}

// NewVM creates a VM instance for an existing VM
func NewVM(ctx context.Context, name string) (*VM, error) {
	runningVMCtx, runningVMCtxCancel := context.WithCancel(context.Background())

	vm := &VM{
		ident:              name,
		runningVMCtx:       runningVMCtx,
		runningVMCtxCancel: runningVMCtxCancel,
		errChan:            make(chan error, 1),
	}

	return vm, nil
}

// CloneVM clones a VM from source to destination
func CloneVM(ctx context.Context, from, to string) error {
	err := Cmd(ctx, nil, "clone", from, to)
	if err != nil {
		return fmt.Errorf("failed to clone VM from %q to %q: %w", from, to, err)
	}
	return nil
}
