# ttab — Terminal Tagger

Dynamically color your terminal tabs and set titles based on your current directory. Zero-latency, zero-dependency, cross-platform.

`ttab` injects a lightweight hook into your shell prompt. When you `cd` into a tagged directory, it instantly recolors the tab and updates the title — so you always know where you are at a glance.

## Supported Terminals

| Terminal | Tab Color | Text Contrast | Title |
|---|---|---|---|
| Windows Terminal | ✅ | ✅ | ✅ |
| iTerm2 | ✅ | auto | ✅ |
| GNOME Terminal | ✅ | ✅ | ✅ |
| Other (OSC-compatible) | ✅ | ✅ | ✅ |

## Install

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/dpkay-io/ttab/main/install.ps1 | iex
```

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/dpkay-io/ttab/main/install.sh | sh
```

## Usage

### Tag a directory

Navigate to any directory and tag it with a color:

```bash
ttab set --color "#ff6b6b" --title "API"
```

The `--title` flag is optional. If omitted, the folder name is used automatically.
If the directory is already tagged or if it inherits a color from a parent directory, the `--color` flag is also optional. You can just set a new title:

```bash
ttab set --title "Just Title"
```

### Clear a tag

Remove the color tag from the current directory:

```bash
ttab clear
```

You can also clear a specific directory without navigating to it:

```bash
ttab clear ~/projects/api
```

### List tags

View all currently tagged directories:

```bash
ttab list
```

### How it works

Once installed, `ttab` adds a tiny hook to your shell prompt. Every time you change directories, it:

1. Checks `~/.terminal_tagger.json` for a matching path
2. Walks parent directories for inherited rules (tag `~/projects` and every subdirectory inherits it)
3. Calculates optimal text contrast (white or black) using W3C luminance
4. Emits escape sequences to color the tab and set the title

### Examples

```bash
# Tag your API project red
cd ~/projects/api
ttab set --color "#ff6b6b" --title "API"

# Tag all projects with a base color (child dirs inherit)
cd ~/projects
ttab set --color "#4ecdc4" --title "Projects"

# Clear tagging
ttab clear

# Manually trigger the hook (usually automatic)
ttab hook /path/to/directory
```

## Configuration

Tags are stored in `~/.config/ttab/config.json` (Linux/macOS) or `%APPDATA%\ttab\config.json` (Windows):

```json
{
  "/home/user/projects": {
    "color": "#4ecdc4",
    "title": "Projects"
  },
  "/home/user/projects/api": {
    "color": "#ff6b6b",
    "title": "API"
  }
}
```

You can edit this file directly if you prefer, or use the `ttab list` command to view it.

## Shell Support

| Shell | Profile | Hook Method |
|---|---|---|
| PowerShell | `$PROFILE` | `prompt` function wrapper |
| Zsh | `~/.zshrc` | `precmd` via `add-zsh-hook` |
| Bash | `~/.bashrc` | `PROMPT_COMMAND` |

## Building from Source

```bash
go build -ldflags="-s -w" -o ttab .
```

## License

MIT
