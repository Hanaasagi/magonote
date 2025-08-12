<h1 align="center"> magonote 🦯 </h1>

<div align="center">
  
𝑨 𝒎𝒐𝒅𝒆𝒓𝒏 𝒕𝒎𝒖𝒙 𝒑𝒍𝒖𝒈𝒊𝒏 𝒕𝒐 𝒔𝒆𝒍𝒆𝒄𝒕 𝒂𝒏𝒅 𝒂𝒄𝒕 𝒐𝒏 𝒗𝒊𝒔𝒊𝒃𝒍𝒆 𝒕𝒆𝒙𝒕 𝒘𝒊𝒕𝒉 𝒎𝒊𝒏𝒊𝒎𝒂𝒍 𝒌𝒆𝒚𝒔𝒕𝒓𝒐𝒌𝒆𝒔
</div>

[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](https://github.com/Hanaasagi/magonote/actions/workflows/ci.yaml/badge.svg)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.24-blue.svg)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-orange.svg)](https://opensource.org/licenses/MIT)

## 🚀 Description

**magonote** is a powerful tmux plugin designed to revolutionize how you interact with text in terminal sessions. It intelligently identifies and highlights various text patterns—such as IP addresses, URLs, file paths, Git hashes, and more—allowing you to quickly select and act on them with minimal effort.



## ✨ Key Features

### 🎯 **Smart Pattern Recognition**
- **Built-in Patterns**: Automatically detects IPs, URLs, file paths, Git hashes, UUIDs, Docker image IDs, and more
- **Custom Patterns**: Add your own regex patterns for project-specific needs
- **Git Integration**: Enhanced support for Git diffs, status output, and commit hashes

### 🎨 **Flexible Customization**
- **Multiple Keyboard Layouts**: Support for QWERTY, QWERTZ, AZERTY, Colemak, Dvorak, and variations
- **Color Themes**: Fully customizable color schemes for hints and highlights  
- **Positioning Options**: Choose hint placement (left/right) for optimal visibility

### 🔧 **Advanced Functionality**
- **Multi-Selection Mode**: Select multiple items at once with visual feedback
- **Editor Integration**: Automatically open files in your preferred editor
- **Contrast Mode**: Enhanced visibility with bracketed hints
- **Unique Filtering**: Options to handle duplicate matches intelligently

## 📸 Screenshots

![preview](https://github.com/user-attachments/assets/98e3bf35-5397-4049-8f19-e21a99ecfe84)



## 📦 Installation

### Prerequisites

- **tmux** version 3.1 or higher
- **Go** 1.21 or higher (for building from source)

### Quick Install

```bash
# Clone the repository
git clone https://github.com/Hanaasagi/magonote ~/.tmux/plugins/tmux-magonote

# Navigate to the plugin directory
cd ~/.tmux/plugins/tmux-magonote

# Build the plugin
make build
```

### Enable in tmux

Add the following line to your `~/.tmux.conf`:

```bash
set -g @plugin 'Hanaasagi/tmux-magonote'
```

Then reload your tmux configuration:

```bash
# Reload tmux config
tmux source-file ~/.tmux.conf
```

### Alternative: Manual Installation

If you prefer manual installation:

```bash
# Clone to a custom location
git clone https://github.com/Hanaasagi/magonote /path/to/magonote
cd /path/to/magonote
make build

# Add to your tmux config
echo "run-shell '/path/to/magonote/magonote.tmux'" >> ~/.tmux.conf
```


## 🎮 Usage

### Basic Usage

Once installed, activate magonote in any tmux session:

```bash
# Default activation (prefix + Space)
<prefix> + Space
```

### Pattern Examples

magonote automatically recognizes these patterns:

| Pattern Type | Example |
|-------------|---------|
| **IPv4** | `192.168.1.1`, `10.0.0.1:8080` |
| **IPv6** | `2001:db8::1`, `[::1]:8080` |
| **URLs** | `https://example.com`, `git@github.com:user/repo.git` |
| **File Paths** | `/home/user/file.txt`, `./config/app.toml` |
| **Git Hashes** | `a1b2c3d`, `1234567890abcdef...` |
| **UUIDs** | `550e8400-e29b-41d4-a716-446655440000` |
| **Docker** | `sha256:30557a29d5abc51e...` |
| **Colors** | `#FF0000`, `#00FF00` |
| **Dates** | `2023-12-01`, `2024-01-15T10:30:45Z` |

---

## ⚙️ Configuration

### Configuration File

Create a configuration file at `~/.config/magonote/config.toml`:

```toml
[core]
# Sets the alphabet used for generating hints
alphabet = "qwerty"

# Output format for the picked hint (%H = hint text, %U = uppercase flag)
format = "%H"

# Hint position: "left", "right", "off_left", or "off_right"
position = "left"

# Enable multi-selection mode
multi = false

# Reverse the order for assigned hints
reverse = false

# Unique level: 0 = none, 1 = unique hints, 2 = highlight only one duplicate
unique_level = 0

# Put square brackets around hint for visibility
contrast = false

[rules]
# User-defined matching and filtering rules

[rules.include]
# Additional rules to match. Only { type = "regex" } is honored here.
rules = [
    # { type = "regex", pattern = "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b" },  # Email
    # { type = "regex", pattern = "\\bhttps?://[\\w.-]+\\b" },                                 # URL
]

[rules.exclude]
# Exclusion rules to filter out unwanted matches by defining regions in the original text
# The rules first identify exclusion regions in the input text, then filter out any matches
# whose coordinates overlap with these regions
rules = [
    # Example: Exclude entire shell prompt lines
    # { type = "regex", pattern = "^user@hostname:.*\\$ " },  # Lines starting with prompt
    # { type = "text", pattern = "~/project$ " },             # Specific prompt fragment

    # Additional examples:
    # { type = "text", pattern = "DEBUG" },                   # Regions containing "DEBUG"
    # { type = "regex", pattern = "^\\[.*\\]\\$ " },         # Bracketed prompts like "[user@host]$ "
    # { type = "regex", pattern = "^\\d{4}-\\d{2}-\\d{2}" }, # Lines starting with dates
    # { type = "regex", pattern = "Error:.*" },              # Entire error message lines

    # Exclude log timestamps and noise:
    # { type = "regex", pattern = "^\\d{2}:\\d{2}:\\d{2}" }, # Timestamps like 12:34:56
    # { type = "text", pattern = "INFO" },                   # Any region containing INFO logs
]

[colors.match]
# Foreground color for matches
foreground = "green"
# Background color for matches
background = "black"

[colors.hint]
# Foreground color for hints
foreground = "yellow"
# Background color for hints
background = "black"

[colors.multi]
# Foreground color for multi selected items
foreground = "yellow"
# Background color for multi selected items
background = "black"

[colors.select]
# Foreground color for selection
foreground = "blue"
# Background color for selection
background = "black"

[plugins.tabledetection]
enabled = true
min_lines = 3
min_columns = 3
confidence_threshold = 0.8

[plugins.colordetection]
enabled = true
```

### Command Line Options

```
Usage:
  magonote [flags]

Flags:
  -a, --alphabet string          Sets the alphabet (default "qwerty")
      --bg-color string          Sets the background color for matches (default "black")
      --config string            Config file path (default: XDG config dir, use 'NONE' to disable)
  -c, --contrast                 Put square brackets around hint for visibility
      --fg-color string          Sets the foreground color for matches (default "green")
  -f, --format string            Specifies the out format for the picked hint (default "%H")
  -h, --help                     help for magonote
      --hint-bg-color string     Sets the background color for hints (default "black")
      --hint-fg-color string     Sets the foreground color for hints (default "yellow")
  -i, --input-file string        Read input from file instead of stdin
  -m, --multi                    Enable multi-selection
      --multi-bg-color string    Sets the background color for multi selected items (default "black")
      --multi-fg-color string    Sets the foreground color for multi selected items (default "yellow")
  -p, --position string          Hint position (default "left")
  -x, --regexp stringArray       Use this regexp as extra pattern to match
  -r, --reverse                  Reverse the order for assigned hints
      --select-bg-color string   Sets the background color for selection (default "black")
      --select-fg-color string   Sets the foreground color for selection (default "blue")
  -t, --target string            Stores the hint in the specified path
  -u, --unique count             Don't show duplicated hints for the same match (use -u for unique hints, -uu for unique match)
  -v, --version                  Print version and exit
```

### Keyboard Layout Options

Available layouts: `qwerty`, `qwertz`, `azerty`, `colemak`, `dvorak`

Each layout also supports hand-specific variants:
- `qwerty-left-hand` - Optimized for left hand
- `qwerty-right-hand` - Optimized for right hand  
- `qwerty-homerow` - Only homerow keys



## 🔗 Alternative Projects

- **[tmux-fingers](https://github.com/Morantron/tmux-fingers)** - Original Ruby/Crystal implementation
- **[tmux-thumbs](https://github.com/fcsonline/tmux-thumbs)** - Rust-based alternative


## 📄 License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.
