# Clipboard Package

A unified Go library for copying text to various clipboard targets including tmux buffers, system clipboard, and remote terminals via OSC52.

## Features

- **Multi-target support**: Copy to tmux buffer, system clipboard, and remote terminals
- **OSC52 integration**: Copy to clipboard over SSH and remote sessions  
- **Cross-platform**: Supports macOS, Linux, and Windows
- **Flexible API**: Choose specific targets or use all available ones

## Quick Start

```go
package main

import (
    "log"
    "github.com/Hanaasagi/magonote/pkg/clipboard"
)

func main() {
    // Copy to all available targets
    err := clipboard.Copy("Hello, World!")
    if err != nil {
        log.Fatal(err)
    }
}
```

## API Overview

### Basic Usage

```go
// Copy to all available clipboard targets
err := clipboard.Copy("text")

// Copy to specific targets
err := clipboard.CopyToTmux("tmux only")
err := clipboard.CopyToSystem("system only") 
err := clipboard.CopyWithOSC52("remote via OSC52")
```

### Advanced Configuration

```go
// Create clipboard with custom options
c := clipboard.New(
    clipboard.WithTmux(true),        // Enable tmux buffer
    clipboard.WithSystem(false),     // Disable system clipboard
    clipboard.WithOSC52(true),       // Enable OSC52 sequences
    clipboard.WithOutput(os.Stderr), // Set OSC52 output destination
)

err := c.Copy("configured copy")
```

### Individual Writers

```go
// Tmux-only writer
tmuxWriter := clipboard.NewTmuxWriter()
err := tmuxWriter.Write("tmux content")

// System-only writer  
systemWriter := clipboard.NewSystemWriter()
err := systemWriter.Write("system content")

// OSC52-only writer
osc52Writer := clipboard.NewOSC52Writer(os.Stderr)
err := osc52Writer.Write("remote content")
```

## Clipboard Targets

### Tmux Buffer

Copies text to the tmux paste buffer. Works when running inside a tmux session.

- Uses `tmux load-buffer` command
- Automatically detects tmux environment via `$TMUX` variable
- Clears buffer when copying empty string

### System Clipboard

Copies to the operating system clipboard using platform-specific tools:

- **macOS**: `pbcopy`
- **Linux**: `wl-copy` (Wayland), `xclip`, `xsel` (X11)  
- **Windows**: `clip`

### OSC52 Terminal Sequences

Copies via ANSI OSC52 escape sequences. Works over SSH and remote connections.

- Encodes text in base64
- Sends escape sequence to terminal
- Automatically wraps in tmux DCS passthrough when in tmux
- Ideal for remote sessions where system tools aren't available

## Environment Detection

```go
// Check what's available
available := clipboard.Available()
fmt.Printf("Tmux: %v\n", available["tmux"])
fmt.Printf("System: %v\n", available["system"])  
fmt.Printf("OSC52: %v\n", available["osc52"])

// Individual checks
if clipboard.IsTmuxSession() {
    // Running in tmux
}

if clipboard.HasSystemClipboard() {
    // System tools available
}
```

## Remote SSH Usage

For remote SSH sessions, OSC52 is typically the best option:

```go
// Detect remote session
isRemote := os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != ""

var c *clipboard.Clipboard
if isRemote {
    // Prefer OSC52 for remote sessions
    c = clipboard.New(
        clipboard.WithOSC52(true),
        clipboard.WithSystem(false),
        clipboard.WithTmux(clipboard.IsTmuxSession()),
    )
} else {
    // Use all methods for local sessions
    c = clipboard.New()
}

err := c.Copy("remote clipboard content")
```

## Custom Writers

Implement the `Writer` interface for custom clipboard destinations:

```go
type Writer interface {
    Write(text string) error
}

type CustomWriter struct{}

func (w *CustomWriter) Write(text string) error {
    // Custom clipboard logic
    return nil
}
```


## License

MIT License - see LICENSE file for details. 
