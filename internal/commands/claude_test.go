package commands

import (
	"path/filepath"
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
