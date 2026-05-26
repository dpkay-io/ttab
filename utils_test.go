package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseHex(t *testing.T) {
	r, g, b := parseHex("#ff6b6b")
	if r != 255 || g != 107 || b != 107 {
		t.Errorf("Expected 255, 107, 107, got %d, %d, %d", r, g, b)
	}

	r, g, b = parseHex("000000")
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("Expected 0, 0, 0, got %d, %d, %d", r, g, b)
	}
}

func TestContrastColor(t *testing.T) {
	// Dark background -> white text
	r, g, b := contrastColor(0, 0, 0)
	if r != 255 || g != 255 || b != 255 {
		t.Errorf("Expected white text for dark bg, got %d, %d, %d", r, g, b)
	}

	// Light background -> black text
	r, g, b = contrastColor(255, 255, 255)
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("Expected black text for light bg, got %d, %d, %d", r, g, b)
	}
}

func TestDetectTerminal(t *testing.T) {
	os.Setenv("WT_SESSION", "1")
	if detectTerminal() != termWindowsTerminal {
		t.Errorf("Expected termWindowsTerminal")
	}
	os.Unsetenv("WT_SESSION")

	os.Setenv("TERM_PROGRAM", "iTerm.app")
	if detectTerminal() != termITerm2 {
		t.Errorf("Expected termITerm2")
	}
	os.Unsetenv("TERM_PROGRAM")

	if detectTerminal() != termStandard {
		t.Errorf("Expected termStandard")
	}
}

func TestColorSequences(t *testing.T) {
	os.Setenv("TERM_PROGRAM", "iTerm.app")
	seq := colorSequences(255, 0, 0, 255, 255, 255, "Test")
	if !strings.Contains(seq, "bg;red;brightness;255") {
		t.Errorf("Expected iTerm2 sequence, got %q", seq)
	}
	os.Unsetenv("TERM_PROGRAM")

	os.Setenv("WT_SESSION", "1")
	seq = colorSequences(255, 0, 0, 255, 255, 255, "Test")
	if !strings.Contains(seq, "4;264;rgb:ff/00/00") {
		t.Errorf("Expected Windows Terminal sequence, got %q", seq)
	}
	os.Unsetenv("WT_SESSION")

	seq = colorSequences(255, 0, 0, 255, 255, 255, "Test")
	if !strings.Contains(seq, "4;264;rgb:ff/00/00") {
		t.Errorf("Expected standard OSC 4 sequence, got %q", seq)
	}
}

func TestResetSequences(t *testing.T) {
	os.Setenv("TERM_PROGRAM", "iTerm.app")
	seq := resetSequences()
	if !strings.Contains(seq, "bg;*;default") {
		t.Errorf("Expected iTerm2 reset, got %q", seq)
	}
	os.Unsetenv("TERM_PROGRAM")

	seq = resetSequences()
	if !strings.Contains(seq, "104;263;264") {
		t.Errorf("Expected standard reset, got %q", seq)
	}
}

func TestDetectShell(t *testing.T) {
	// We can't strictly test detectShell on Windows vs Linux completely because of runtime.GOOS,
	// but we can test environment variable parsing if not windows
	if runtimeOS := "linux"; runtimeOS == "linux" { // simulating non-windows behavior testing logic
		os.Setenv("SHELL", "/bin/zsh")
		if detectShell() != "zsh" && detectShell() != "powershell" { // powershell is forced on windows
			t.Errorf("Expected zsh")
		}
	}
}

func TestDefaultProfilePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	
	ps := defaultProfilePath("powershell")
	if ps != filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1") {
		t.Errorf("Unexpected powershell profile path: %s", ps)
	}

	zsh := defaultProfilePath("zsh")
	if zsh != filepath.Join(home, ".zshrc") {
		t.Errorf("Unexpected zsh profile path: %s", zsh)
	}

	bash := defaultProfilePath("bash")
	if bash != filepath.Join(home, ".bashrc") {
		t.Errorf("Unexpected bash profile path: %s", bash)
	}
}

func TestHookSnippetVersion(t *testing.T) {
	for _, shell := range []string{"powershell", "zsh", "bash"} {
		snippet := hookSnippet(shell)
		expected := "# ttag hook v" + version
		if !strings.Contains(snippet, expected) {
			t.Errorf("%s hook missing version line %q", shell, expected)
		}
	}
}

func TestInstalledHookVersion(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "profile")

	// No file → empty
	if v := installedHookVersion("bash", profile); v != "" {
		t.Errorf("Expected empty version for missing profile, got %q", v)
	}

	// Write hook with version
	snippet := hookSnippet("bash")
	os.WriteFile(profile, []byte("# stuff\n"+snippet+"\n"), 0644)
	if v := installedHookVersion("bash", profile); v != version {
		t.Errorf("Expected %q, got %q", version, v)
	}

	// Write hook without version line (simulating old install)
	old := hookMarkerStart + "\nsome old content\n" + hookMarkerEnd
	os.WriteFile(profile, []byte(old), 0644)
	if v := installedHookVersion("bash", profile); v != "" {
		t.Errorf("Expected empty version for old hook, got %q", v)
	}
}

func TestHookSnippet(t *testing.T) {
	ps := hookSnippet("powershell")
	if !strings.Contains(ps, "global:prompt") {
		t.Errorf("Expected PowerShell hook")
	}
	if !strings.Contains(ps, "?1004h") {
		t.Errorf("Expected focus reporting enable in PowerShell hook")
	}
	if !strings.Contains(ps, "__ttag_focused") {
		t.Errorf("Expected focus tracking in PowerShell hook")
	}

	zsh := hookSnippet("zsh")
	if !strings.Contains(zsh, "add-zsh-hook") {
		t.Errorf("Expected Zsh hook")
	}
	if !strings.Contains(zsh, "?1004h") {
		t.Errorf("Expected focus reporting enable in Zsh hook")
	}
	if !strings.Contains(zsh, "__ttag_focus_in") {
		t.Errorf("Expected focus handlers in Zsh hook")
	}
	if !strings.Contains(zsh, "zshexit") {
		t.Errorf("Expected exit cleanup in Zsh hook")
	}

	bash := hookSnippet("bash")
	if !strings.Contains(bash, "PROMPT_COMMAND") {
		t.Errorf("Expected Bash hook")
	}
	if !strings.Contains(bash, "?1004h") {
		t.Errorf("Expected focus reporting enable in Bash hook")
	}
	if !strings.Contains(bash, "__ttag_focus_in") {
		t.Errorf("Expected focus handlers in Bash hook")
	}
	if !strings.Contains(bash, "__ttag_cleanup") {
		t.Errorf("Expected exit cleanup in Bash hook")
	}
}
