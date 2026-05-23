package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get user home dir: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Clean path",
			input:    filepath.Join("some", "path", "dir", ".."),
			expected: filepath.Join("some", "path"),
		},
		{
			name:     "Expand home dir",
			input:    filepath.Join("~", "documents"),
			expected: filepath.Join(home, "documents"),
		},
		{
			name:     "Expand home dir root",
			input:    "~",
			expected: home,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}

	// Windows specific tests
	if runtime.GOOS == "windows" {
		t.Run("Windows drive letter lowercase", func(t *testing.T) {
			input := "c:\\projects\\api"
			expected := "C:\\projects\\api"
			result := normalizePath(input)
			if result != expected {
				t.Errorf("expected %q, got %q", expected, result)
			}
		})
		t.Run("Windows drive letter uppercase", func(t *testing.T) {
			input := "C:\\projects\\api"
			expected := "C:\\projects\\api"
			result := normalizePath(input)
			if result != expected {
				t.Errorf("expected %q, got %q", expected, result)
			}
		})
	}
}

func TestFindConfig(t *testing.T) {
	cfg := map[string]DirConfig{
		filepath.Join("home", "user", "projects"): {Color: "#ff0000", Title: "Projects"},
	}

	// exact match
	dc, ok := findConfig(cfg, filepath.Join("home", "user", "projects"))
	if !ok || dc.Title != "Projects" {
		t.Errorf("expected exact match to succeed")
	}

	// case-insensitive match
	dc, ok = findConfig(cfg, filepath.Join("HOME", "USER", "PROJECTS"))
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		if !ok || dc.Title != "Projects" {
			t.Errorf("expected case-insensitive match to succeed on Windows/macOS")
		}
	} else {
		if ok {
			t.Errorf("expected case-insensitive match to fail on Linux")
		}
	}

	// no match
	_, ok = findConfig(cfg, filepath.Join("home", "user", "other"))
	if ok {
		t.Errorf("expected no match")
	}
}

func TestMatchPath(t *testing.T) {
	rootPath := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		rootPath = "C:\\"
	}

	cfg := map[string]DirConfig{
		filepath.Join(rootPath, "projects"):                   {Color: "#ff0000", Title: "Projects"},
		filepath.Join(rootPath, "projects", "api"):            {Color: "#00ff00", Title: "API"},
		filepath.Join(rootPath, "projects", "api", "v1"):      {Color: "#0000ff", Title: "API v1"},
	}

	tests := []struct {
		name          string
		pwd           string
		expectedMatch bool
		expectedTitle string
	}{
		{
			name:          "Exact match level 1",
			pwd:           filepath.Join(rootPath, "projects"),
			expectedMatch: true,
			expectedTitle: "Projects",
		},
		{
			name:          "Exact match level 2",
			pwd:           filepath.Join(rootPath, "projects", "api"),
			expectedMatch: true,
			expectedTitle: "API",
		},
		{
			name:          "Parent fallback match",
			pwd:           filepath.Join(rootPath, "projects", "api", "v2"),
			expectedMatch: true,
			expectedTitle: "API", // falls back to parent "api"
		},
		{
			name:          "Grandparent fallback match",
			pwd:           filepath.Join(rootPath, "projects", "frontend", "src"),
			expectedMatch: true,
			expectedTitle: "Projects", // falls back to grandparent "projects"
		},
		{
			name:          "No match",
			pwd:           filepath.Join(rootPath, "other", "folder"),
			expectedMatch: false,
		},
		{
			name:          "Root path no match",
			pwd:           rootPath,
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, dc := matchPath(cfg, tt.pwd)
			if match != tt.expectedMatch {
				t.Errorf("expected match %v, got %v", tt.expectedMatch, match)
			}
			if match && dc.Title != tt.expectedTitle {
				t.Errorf("expected title %q, got %q", tt.expectedTitle, dc.Title)
			}
		})
	}

	// Case insensitive test on supported OS
	t.Run("Case insensitive parent fallback", func(t *testing.T) {
		pwd := filepath.Join(rootPath, "PROJECTS", "FRONTEND")
		match, dc := matchPath(cfg, pwd)
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
			if !match || dc.Title != "Projects" {
				t.Errorf("expected case-insensitive fallback match")
			}
		} else {
			if match {
				t.Errorf("expected case-insensitive match to fail on linux")
			}
		}
	})
}
