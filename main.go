package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	hookMarkerStart = "# ttag hook - start"
	hookMarkerEnd   = "# ttag hook - end"
	version         = "1.0.7"
)

// DirConfig stores the appearance settings for a tagged directory.
type DirConfig struct {
	Color string `json:"color,omitempty"`
	Title string `json:"title,omitempty"`
}

// ─── Entrypoint ──────────────────────────────────────────────────────────────

func main() {
	if len(os.Args) < 2 || os.Args[1] != "hook" {
		fmt.Printf("ttag v%s\n", version)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "install":
		cmdInstall(os.Args[2:])
	case "uninstall":
		cmdUninstall(os.Args[2:])
	case "set":
		cmdSet(os.Args[2:])
	case "clear":
		cmdClear(os.Args[2:])
	case "list":
		cmdList()
	case "hook":
		cmdHook(os.Args[2:])
	case "upgrade":
		cmdUpgrade()
	case "version", "--version", "-v":
		// Version is printed in banner above
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`ttag - Terminal Tagger

Dynamically color your terminal tabs based on your current directory.

Usage:
  ttag install [--profile PATH]  Install shell hook into your profile
  ttag uninstall                 Remove shell hook from your profile
  ttag set [--color "#HEX"]      Tag a directory with a tab color
           [--title "Name"]      Optional title prefix
  ttag clear [path]              Remove tagging for the current or specified directory
  ttag list                      List all tagged directories
  ttag upgrade                   Upgrade ttag to the latest version
  ttag hook [path]               (Used by shell hook) Apply colors for a path
  ttag version                   Print version

Examples:
  ttag set --color "#ff6b6b" --title "API"
  ttag set --title "Just Title"
  ttag clear
  ttag clear ~/projects/api`)
}

// ─── Config File ─────────────────────────────────────────────────────────────

func configPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine config directory: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(configDir, "ttag", "config.json")
}

func loadConfig() (map[string]DirConfig, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]DirConfig), nil
		}
		return nil, err
	}
	var cfg map[string]DirConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid JSON in config file: %w", err)
	}
	if cfg == nil {
		cfg = make(map[string]DirConfig)
	}
	return cfg, nil
}

func saveConfig(cfg map[string]DirConfig) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot marshal config: %v\n", err)
		os.Exit(1)
	}
	
	cPath := configPath()
	if err := os.MkdirAll(filepath.Dir(cPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create config directory: %v\n", err)
		os.Exit(1)
	}
	
	if err := os.WriteFile(cPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot write config: %v\n", err)
		os.Exit(1)
	}
}

// ─── set ─────────────────────────────────────────────────────────────────────

func cmdSet(args []string) {
	var color, title string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--color", "-c":
			if i+1 < len(args) {
				i++
				color = args[i]
			}
		case "--title", "-t":
			if i+1 < len(args) {
				i++
				title = args[i]
			}
		}
	}

	if color == "" && title == "" {
		fmt.Fprintln(os.Stderr, "Error: either --color or --title must be provided.\nUsage: ttag set [--color \"#HEX\"] [--title \"Name\"]")
		os.Exit(1)
	}

	if color != "" {
		// Validate hex color
		color = strings.TrimPrefix(color, "#")
		if len(color) != 6 {
			fmt.Fprintln(os.Stderr, "Error: color must be a 6-digit hex value (e.g., #ff6b6b)")
			os.Exit(1)
		}
		if _, err := strconv.ParseUint(color, 16, 32); err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid hex color: #%s\n", color)
			os.Exit(1)
		}
		color = "#" + strings.ToLower(color)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot get working directory: %v\n", err)
		os.Exit(1)
	}
	cwd = normalizePath(cwd)

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	
	dc := cfg[cwd]
	if color != "" {
		dc.Color = color
	}
	if title != "" {
		dc.Title = title
	}
	cfg[cwd] = dc
	
	saveConfig(cfg)
	touchUpdateFile()

	titleDisplay := dc.Title
	if titleDisplay == "" {
		titleDisplay = "(auto)"
	}
	colorDisplay := dc.Color
	if colorDisplay == "" {
		colorDisplay = "(inherited or default)"
	}
	fmt.Printf("Tagged %s  color: %s, title: %s\n", cwd, colorDisplay, titleDisplay)
	cmdHook(nil)
}

// ─── clear ───────────────────────────────────────────────────────────────────

func cmdClear(args []string) {
	var targetPath string
	if len(args) > 0 {
		targetPath = args[0]
		// Convert relative to absolute
		if abs, err := filepath.Abs(targetPath); err == nil {
			targetPath = abs
		}
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot get working directory: %v\n", err)
			os.Exit(1)
		}
		targetPath = cwd
	}
	targetPath = normalizePath(targetPath)

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	
	if _, ok := cfg[targetPath]; !ok {
		fmt.Println("No tagging found for this directory.")
		return
	}
	delete(cfg, targetPath)
	saveConfig(cfg)
	touchUpdateFile()

	cmdHook(nil)
	
	fmt.Printf("Cleared tagging for %s\n", targetPath)
}

// ─── list ────────────────────────────────────────────────────────────────────

func cmdList() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg) == 0 {
		fmt.Println("No directories are currently tagged.")
		return
	}

	fmt.Println("Tagged directories:")
	for p, dc := range cfg {
		color := dc.Color
		if color == "" {
			color = "none"
		}
		title := dc.Title
		if title == "" {
			title = "(auto)"
		}
		fmt.Printf("  %s\n    color: %s, title: %s\n", p, color, title)
	}
}

// ─── hook ────────────────────────────────────────────────────────────────────

func cmdHook(args []string) {
	var pwd string
	noTitle := false
	titleOnly := false

	for _, arg := range args {
		switch arg {
		case "--no-title":
			noTitle = true
		case "--title-only":
			titleOnly = true
		default:
			if pwd == "" {
				pwd = arg
			}
		}
	}

	if pwd == "" {
		var err error
		pwd, err = os.Getwd()
		if err != nil {
			return // fail silently — this runs on every prompt
		}
	}
	pwd = normalizePath(pwd)

	cfg, err := loadConfig()
	if err != nil {
		// Fail silently during hook to avoid breaking the prompt
		return
	}

	matched, dc := matchPath(cfg, pwd)

	if !matched {
		if !titleOnly {
			// User left a tagged directory — reset tab appearance
			fmt.Print(resetSequences())
		}
		return
	}

	// Build title: "Prefix | LeafFolder" or just "LeafFolder"
	title := filepath.Base(pwd)
	if dc.Title != "" {
		if title != dc.Title {
			title = dc.Title + " | " + title
		} else {
			title = dc.Title
		}
	}

	if titleOnly {
		fmt.Print(title)
		return
	}

	if dc.Color == "" {
		if !noTitle {
			fmt.Printf("\033]0;%s\a", title)
		}
		return
	}

	// Parse background color and compute contrast foreground
	r, g, b := parseHex(dc.Color)
	fr, fg, fb := contrastColor(r, g, b)

	titleArg := title
	if noTitle {
		titleArg = ""
	}
	fmt.Print(colorSequences(r, g, b, fr, fg, fb, titleArg))
}

// ─── Terminal Detection ──────────────────────────────────────────────────────

type termType int

const (
	termStandard termType = iota
	termWindowsTerminal
	termITerm2
)

func detectTerminal() termType {
	// Windows Terminal sets WT_SESSION on every pane
	if os.Getenv("WT_SESSION") != "" {
		return termWindowsTerminal
	}
	// iTerm2 identifies itself via TERM_PROGRAM
	if os.Getenv("TERM_PROGRAM") == "iTerm.app" {
		return termITerm2
	}
	return termStandard
}

// ─── ANSI / OSC Escape Sequences ────────────────────────────────────────────

func colorSequences(r, g, b, fr, fg, fb uint8, title string) string {
	var s strings.Builder
	t := detectTerminal()

	switch t {
	case termITerm2:
		// iTerm2 proprietary: set tab background color per-channel
		fmt.Fprintf(&s, "\033]6;1;bg;red;brightness;%d\a", r)
		fmt.Fprintf(&s, "\033]6;1;bg;green;brightness;%d\a", g)
		fmt.Fprintf(&s, "\033]6;1;bg;blue;brightness;%d\a", b)
		// iTerm2 handles text contrast automatically

	case termWindowsTerminal:
		// Palette index 264 = tab background, 263 = tab foreground
		fmt.Fprintf(&s, "\033]4;264;rgb:%02x/%02x/%02x\a", r, g, b)
		fmt.Fprintf(&s, "\033]4;263;rgb:%02x/%02x/%02x\a", fr, fg, fb)

	default:
		// Standard OSC 4 — works in GNOME Terminal, Konsole, and many others
		fmt.Fprintf(&s, "\033]4;264;rgb:%02x/%02x/%02x\a", r, g, b)
		fmt.Fprintf(&s, "\033]4;263;rgb:%02x/%02x/%02x\a", fr, fg, fb)
	}

	// OSC 0: set window/tab title (universal)
	if title != "" {
		fmt.Fprintf(&s, "\033]0;%s\a", title)
	}

	return s.String()
}

func resetSequences() string {
	var s strings.Builder
	t := detectTerminal()

	switch t {
	case termITerm2:
		s.WriteString("\033]6;1;bg;*;default\a")
	default:
		// OSC 104: reset palette entries 263 and 264
		s.WriteString("\033]104;263;264\a")
	}

	return s.String()
}

// ─── Color Utilities ─────────────────────────────────────────────────────────

// parseHex converts a "#rrggbb" string to individual RGB bytes.
func parseHex(hex string) (uint8, uint8, uint8) {
	hex = strings.TrimPrefix(hex, "#")
	val, _ := strconv.ParseUint(hex, 16, 32)
	return uint8((val >> 16) & 0xFF), uint8((val >> 8) & 0xFF), uint8(val & 0xFF)
}

// contrastColor returns white or black RGB depending on background luminance.
// Uses the W3C perceived-brightness formula.
func contrastColor(r, g, b uint8) (uint8, uint8, uint8) {
	luminance := float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114
	if luminance < 128 {
		return 255, 255, 255 // white text on dark background
	}
	return 0, 0, 0 // black text on light background
}

// ─── Path Utilities ──────────────────────────────────────────────────────────

// normalizePath cleans a filesystem path and expands ~ to the home directory.
func normalizePath(p string) string {
	if strings.HasPrefix(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	p = filepath.Clean(p)

	// Normalize Windows drive letter to uppercase for consistent matching
	if runtime.GOOS == "windows" && len(p) >= 2 && p[1] == ':' {
		p = strings.ToUpper(p[:1]) + p[1:]
	}
	return p
}

// findConfig looks up a path in the config, using case-insensitive matching on Windows/macOS.
func findConfig(cfg map[string]DirConfig, p string) (DirConfig, bool) {
	if dc, ok := cfg[p]; ok {
		return dc, true
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		for k, v := range cfg {
			if strings.EqualFold(k, p) {
				return v, true
			}
		}
	}
	return DirConfig{}, false
}

// matchPath checks if pwd (or any of its parent directories) is tagged.
// It merges properties, so a child can override the title but inherit the color from a parent.
func matchPath(cfg map[string]DirConfig, pwd string) (bool, DirConfig) {
	current := pwd
	var finalTitle string
	var finalColor string
	matched := false

	for {
		if dc, ok := findConfig(cfg, current); ok {
			matched = true
			if finalTitle == "" && dc.Title != "" {
				finalTitle = dc.Title
			}
			if finalColor == "" && dc.Color != "" {
				finalColor = dc.Color
			}
			if finalTitle != "" && finalColor != "" {
				break
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break // reached filesystem root
		}
		current = parent
	}

	if !matched {
		return false, DirConfig{}
	}
	return true, DirConfig{Color: finalColor, Title: finalTitle}
}

// ─── install ─────────────────────────────────────────────────────────────────

func cmdInstall(args []string) {
	shell := detectShell()

	// Allow install scripts to pass the exact profile path
	var profileOverride string
	for i := 0; i < len(args); i++ {
		if args[i] == "--profile" && i+1 < len(args) {
			profileOverride = args[i+1]
			break
		}
	}

	profilePath := profileOverride
	if profilePath == "" {
		profilePath = defaultProfilePath(shell)
	}

	fmt.Printf("Detected shell: %s\n", shell)
	fmt.Printf("Profile: %s\n", profilePath)

	hookCode := hookSnippet(shell)

	// Read existing profile (may not exist yet)
	existing, _ := os.ReadFile(profilePath)
	content := string(existing)

	if strings.Contains(content, hookMarkerStart) {
		// Replace existing hook block (supports re-install / upgrade)
		start := strings.Index(content, hookMarkerStart)
		end := strings.Index(content, hookMarkerEnd)
		if end > start {
			end += len(hookMarkerEnd)
			content = content[:start] + hookCode + content[end:]
		}
	} else {
		// Append hook to end of profile
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n" + hookCode + "\n"
	}

	// Ensure the profile directory exists
	if err := os.MkdirAll(filepath.Dir(profilePath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create profile directory: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot write profile: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Shell hook installed successfully!")
	fmt.Println("  Restart your terminal or source your profile to activate.")
}

func cmdUninstall(args []string) {
	shell := detectShell()

	// Allow uninstall scripts to pass the exact profile path
	var profileOverride string
	for i := 0; i < len(args); i++ {
		if args[i] == "--profile" && i+1 < len(args) {
			profileOverride = args[i+1]
			break
		}
	}

	profilePath := profileOverride
	if profilePath == "" {
		profilePath = defaultProfilePath(shell)
	}

	existing, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No shell profile found. Nothing to uninstall.")
			return
		}
		fmt.Fprintf(os.Stderr, "Error reading profile: %v\n", err)
		os.Exit(1)
	}

	content := string(existing)
	if !strings.Contains(content, hookMarkerStart) {
		fmt.Println("No ttag hook found in profile. Nothing to uninstall.")
		return
	}

	start := strings.Index(content, hookMarkerStart)
	end := strings.Index(content, hookMarkerEnd) + len(hookMarkerEnd)

	// Try to include the preceding newline if one exists so we don't leave blank lines
	if start > 0 && content[start-1] == '\n' {
		start--
		if start > 0 && content[start-1] == '\r' {
			start--
		}
	}

	content = content[:start] + content[end:]

	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot write profile: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Shell hook removed successfully from", profilePath)
	fmt.Println("To completely remove ttag:")
	fmt.Println("  1. Remove the ~/.terminal_tagger directory")
	fmt.Println("  2. Remove ~/.terminal_tagger/bin from your PATH")
}

func detectShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	shell := os.Getenv("SHELL")
	if strings.Contains(shell, "zsh") {
		return "zsh"
	}
	return "bash"
}

func defaultProfilePath(shell string) string {
	home, _ := os.UserHomeDir()
	switch shell {
	case "powershell":
		// Default PowerShell 7+ profile location
		return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	case "zsh":
		return filepath.Join(home, ".zshrc")
	default:
		return filepath.Join(home, ".bashrc")
	}
}

// hookSnippet returns the shell-specific code block injected into the profile.
func hookSnippet(shell string) string {
	switch shell {
	case "powershell":
		return hookMarkerStart + "\n" +
			"$__ttag_orig_prompt = $function:prompt\n" +
			"function global:prompt {\n" +
			"    $origExit = $LASTEXITCODE\n" +
			"    $currentPwd = $executionContext.SessionState.Path.CurrentLocation.Path\n" +
			"    $updateFile = \"$HOME\\.ttag_update\"\n" +
			"    $updateTime = $null\n" +
			"    if (Test-Path $updateFile) { $updateTime = (Get-Item $updateFile).LastWriteTime }\n" +
			"    if ($global:__ttag_last_pwd -ne $currentPwd -or $global:__ttag_last_update -ne $updateTime) {\n" +
			"        $__ttag_o = & ttag hook --no-title \"$currentPwd\" 2>$null\n" +
			"        if ($__ttag_o) { [Console]::Write($__ttag_o -join '') }\n" +
			"        $global:__ttag_title_prefix = (& ttag hook --title-only \"$currentPwd\" 2>$null) -join ''\n" +
			"        $global:__ttag_last_pwd = $currentPwd\n" +
			"        $global:__ttag_last_update = $updateTime\n" +
			"    }\n" +
			"    if ($global:__ttag_title_prefix) { $Host.UI.RawUI.WindowTitle = $global:__ttag_title_prefix }\n" +
			"    $global:LASTEXITCODE = $origExit\n" +
			"    if ($__ttag_orig_prompt) { & $__ttag_orig_prompt } else { \"PS $($executionContext.SessionState.Path.CurrentLocation)$('>' * ($nestedPromptLevel + 1)) \" }\n" +
			"}\n" +
			hookMarkerEnd

	case "zsh":
		return hookMarkerStart + "\n" +
			"__ttag_hook() {\n" +
			"  local update_file=\"$HOME/.ttag_update\"\n" +
			"  local update_time=\"\"\n" +
			"  if [ -f \"$update_file\" ]; then\n" +
			"    if [[ \"$OSTYPE\" == \"darwin\"* ]]; then\n" +
			"      update_time=$(stat -f %m \"$update_file\" 2>/dev/null)\n" +
			"    else\n" +
			"      update_time=$(stat -c %Y \"$update_file\" 2>/dev/null)\n" +
			"    fi\n" +
			"  fi\n" +
			"  if [[ \"$PWD\" != \"$__ttag_last_pwd\" ]] || [[ \"$update_time\" != \"$__ttag_last_update\" ]]; then\n" +
			"    ttag hook --no-title \"$PWD\"\n" +
			"    __ttag_title_prefix=$(ttag hook --title-only \"$PWD\" 2>/dev/null)\n" +
			"    export __ttag_last_pwd=\"$PWD\"\n" +
			"    export __ttag_last_update=\"$update_time\"\n" +
			"  fi\n" +
			"  if [[ -n \"$__ttag_title_prefix\" ]]; then\n" +
			"    printf '\\033]0;%s\\a' \"$__ttag_title_prefix\"\n" +
			"  fi\n" +
			"}\n" +
			"autoload -Uz add-zsh-hook\n" +
			"add-zsh-hook precmd __ttag_hook\n" +
			hookMarkerEnd

	default: // bash
		return hookMarkerStart + "\n" +
			"__ttag_hook() {\n" +
			"  local update_file=\"$HOME/.ttag_update\"\n" +
			"  local update_time=\"\"\n" +
			"  if [ -f \"$update_file\" ]; then\n" +
			"    if [ \"$(uname)\" = \"Darwin\" ]; then\n" +
			"      update_time=$(stat -f %m \"$update_file\" 2>/dev/null)\n" +
			"    else\n" +
			"      update_time=$(stat -c %Y \"$update_file\" 2>/dev/null)\n" +
			"    fi\n" +
			"  fi\n" +
			"  if [ \"$PWD\" != \"$__ttag_last_pwd\" ] || [ \"$update_time\" != \"$__ttag_last_update\" ]; then\n" +
			"    ttag hook --no-title \"$PWD\"\n" +
			"    __ttag_title_prefix=$(ttag hook --title-only \"$PWD\" 2>/dev/null)\n" +
			"    export __ttag_last_pwd=\"$PWD\"\n" +
			"    export __ttag_last_update=\"$update_time\"\n" +
			"  fi\n" +
			"  if [ -n \"$__ttag_title_prefix\" ]; then\n" +
			"    printf '\\033]0;%s\\a' \"$__ttag_title_prefix\"\n" +
			"  fi\n" +
			"}\n" +
			"PROMPT_COMMAND=\"__ttag_hook${PROMPT_COMMAND:+;$PROMPT_COMMAND}\"\n" +
			hookMarkerEnd
	}
}

// ─── Touch Update File ────────────────────────────────────────────────────────

func touchUpdateFile() {
	home, err := os.UserHomeDir()
	if err == nil {
		path := filepath.Join(home, ".ttag_update")
		os.WriteFile(path, []byte("1"), 0644)
		now := time.Now()
		os.Chtimes(path, now, now)
	}
}

// ─── upgrade ─────────────────────────────────────────────────────────────────

func cmdUpgrade() {
	fmt.Println("Checking for upgrades...")
	
	resp, err := http.Get("https://api.github.com/repos/dpkay-io/ttag/releases/latest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not reach GitHub API: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: GitHub API returned status %s\n", resp.Status)
		os.Exit(1)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to parse GitHub API response: %v\n", err)
		os.Exit(1)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion == version {
		fmt.Printf("You are already using the latest version (v%s).\n", version)
		return
	}

	fmt.Printf("Upgrade available: v%s -> v%s\n", version, latestVersion)

	osName := runtime.GOOS
	archName := runtime.GOARCH
	
	assetName := fmt.Sprintf("ttag-%s-%s", osName, archName)
	if osName == "windows" {
		assetName += ".exe"
	}

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		fmt.Fprintf(os.Stderr, "Error: No suitable binary found for your OS/Arch (%s) in the latest release.\n", assetName)
		os.Exit(1)
	}

	fmt.Printf("Downloading %s...\n", assetName)
	
	dlResp, err := http.Get(downloadURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to download binary: %v\n", err)
		os.Exit(1)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: Download failed with status %s\n", dlResp.Status)
		os.Exit(1)
	}

	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not determine executable path: %v\n", err)
		os.Exit(1)
	}

	newPath := execPath + ".new"
	oldPath := execPath + ".old"

	out, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create temporary file: %v\n", err)
		os.Exit(1)
	}

	if _, err := io.Copy(out, dlResp.Body); err != nil {
		out.Close()
		fmt.Fprintf(os.Stderr, "Error: Failed to write downloaded data: %v\n", err)
		os.Exit(1)
	}
	out.Close()

	// Swap binaries
	_ = os.Remove(oldPath) // remove previous old file if exists
	if err := os.Rename(execPath, oldPath); err != nil {
		os.Remove(newPath)
		fmt.Fprintf(os.Stderr, "Error: Failed to rename current executable: %v\n", err)
		os.Exit(1)
	}

	if err := os.Rename(newPath, execPath); err != nil {
		// Rollback on failure
		os.Rename(oldPath, execPath)
		fmt.Fprintf(os.Stderr, "Error: Failed to replace executable: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Upgrade successful!")
}
