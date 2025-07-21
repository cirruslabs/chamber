package ssh

import (
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestNewTerminal(t *testing.T) {
	// Create a mock SSH client (nil is fine for this basic test)
	var client *ssh.Client

	terminal := NewTerminal(client)

	if terminal == nil {
		t.Fatal("NewTerminal returned nil")
	}

	if terminal.client != client {
		t.Fatal("Terminal client not set correctly")
	}
}

func TestTerminalStructure(t *testing.T) {
	// Test that the Terminal struct has the expected methods
	var client *ssh.Client
	terminal := NewTerminal(client)

	// Test that the terminal has the expected structure
	if terminal.client != client {
		t.Fatal("Terminal client not set correctly")
	}

	// We can't test RunInteractiveCommand with a nil client as it will panic
	// But we can verify the method exists by checking it compiles
}
