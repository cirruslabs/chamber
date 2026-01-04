package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectoryNameExtraction(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple directory name",
			path:     "/Users/fedor/Documents/chamber",
			expected: "chamber",
		},
		{
			name:     "nested directory",
			path:     "/Users/fedor/workspace/my-project",
			expected: "my-project",
		},
		{
			name:     "root directory",
			path:     "/",
			expected: "/",
		},
		{
			name:     "current directory",
			path:     ".",
			expected: ".",
		},
		{
			name:     "directory with spaces",
			path:     "/Users/fedor/My Documents/test project",
			expected: "test project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filepath.Base(tt.path)
			if result != tt.expected {
				t.Errorf("filepath.Base(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestParseDirectoryMounts(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	tests := []struct {
		name        string
		dirs        []string
		wantNames   []string
		wantRO      []bool
		wantErr     bool
		errContains string
	}{
		{
			name:      "single directory",
			dirs:      []string{"data:/path/to/data"},
			wantNames: []string{"data"},
			wantRO:    []bool{false},
			wantErr:   false,
		},
		{
			name:      "multiple directories",
			dirs:      []string{"data:/path/to/data", "docs:/path/to/docs"},
			wantNames: []string{"data", "docs"},
			wantRO:    []bool{false, false},
			wantErr:   false,
		},
		{
			name:      "read-only directory",
			dirs:      []string{"docs:/path/to/docs:ro"},
			wantNames: []string{"docs"},
			wantRO:    []bool{true},
			wantErr:   false,
		},
		{
			name:      "mixed read-write and read-only",
			dirs:      []string{"data:/path/to/data", "docs:/path/to/docs:ro"},
			wantNames: []string{"data", "docs"},
			wantRO:    []bool{false, true},
			wantErr:   false,
		},
		{
			name:      "tilde expansion",
			dirs:      []string{"home:~/test-dir"},
			wantNames: []string{"home"},
			wantRO:    []bool{false},
			wantErr:   false,
		},
		{
			name:        "invalid format - missing path",
			dirs:        []string{"data"},
			wantErr:     true,
			errContains: "invalid --dir format",
		},
		{
			name:        "empty mount name",
			dirs:        []string{":/path/to/data"},
			wantErr:     true,
			errContains: "mount name cannot be empty",
		},
		{
			name:        "whitespace-only mount name",
			dirs:        []string{"  :/path/to/data"},
			wantErr:     true,
			errContains: "mount name cannot be empty",
		},
		{
			name:        "duplicate mount names",
			dirs:        []string{"data:/path1", "data:/path2"},
			wantErr:     true,
			errContains: "duplicate mount name",
		},
		{
			name:        "invalid read-only flag",
			dirs:        []string{"data:/path:rw"},
			wantErr:     true,
			errContains: "must be 'ro'",
		},
		{
			name:        "invalid read-only flag - readonly",
			dirs:        []string{"data:/path:readonly"},
			wantErr:     true,
			errContains: "must be 'ro'",
		},
		{
			name:      "just home directory",
			dirs:      []string{"home:~"},
			wantNames: []string{"home"},
			wantRO:    []bool{false},
			wantErr:   false,
		},
		{
			name:      "empty dirs",
			dirs:      []string{},
			wantNames: nil,
			wantRO:    nil,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounts, err := parseDirectoryMounts(tt.dirs)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDirectoryMounts() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("parseDirectoryMounts() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("parseDirectoryMounts() unexpected error: %v", err)
				return
			}

			if len(mounts) != len(tt.wantNames) {
				t.Errorf("parseDirectoryMounts() got %d mounts, want %d", len(mounts), len(tt.wantNames))
				return
			}

			for i, mount := range mounts {
				if mount.Name != tt.wantNames[i] {
					t.Errorf("mount[%d].Name = %q, want %q", i, mount.Name, tt.wantNames[i])
				}
				if mount.ReadOnly != tt.wantRO[i] {
					t.Errorf("mount[%d].ReadOnly = %v, want %v", i, mount.ReadOnly, tt.wantRO[i])
				}
			}
		})
	}

	// Test tilde expansion separately
	t.Run("tilde expansion produces correct path", func(t *testing.T) {
		mounts, err := parseDirectoryMounts([]string{"test:~/some-dir"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mounts) != 1 {
			t.Fatalf("expected 1 mount, got %d", len(mounts))
		}
		expectedPath := filepath.Join(homeDir, "some-dir")
		if mounts[0].Path != expectedPath {
			t.Errorf("mount.Path = %q, want %q", mounts[0].Path, expectedPath)
		}
	})

	// Test just ~ expands to home directory
	t.Run("just tilde expands to home directory", func(t *testing.T) {
		mounts, err := parseDirectoryMounts([]string{"home:~"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mounts) != 1 {
			t.Fatalf("expected 1 mount, got %d", len(mounts))
		}
		if mounts[0].Path != homeDir {
			t.Errorf("mount.Path = %q, want %q", mounts[0].Path, homeDir)
		}
	})
}
