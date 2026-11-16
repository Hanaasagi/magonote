# Clipboard Package

A reusable, stdlib-only Go library for copying text to tmux buffers, the system
clipboard, and remote terminals via OSC 52.

Application-specific routing policy (which mechanism to use when) lives outside
this package; the library provides the mechanisms and convenient defaults.

## Features

- **Multi-target support**: tmux buffer, system clipboard, and OSC 52
- **OSC 52 integration**: copy to the clipboard over SSH and remote sessions
- **Cross-platform**: macOS, Linux, and Windows
- **Safe defaults**: empty text is a no-op (it never clears the clipboard)

## Quick Start

```go
package main

import (
    "log"
    "github.com/Hanaasagi/magonote/pkg/clipboard"
)

func main() {
    err := clipboard.Copy("Hello, World!")
    if err != nil {
        log.Fatal(err)
    }
}
```

## Low-level mechanisms

These are the reusable building blocks. Prefer them when integrating into a
host application that supplies its own tmux client.

```go
// Pure OSC 52 encoding (no tmux knowledge, no DCS wrapping).
// Returns nil for empty text.
seq := clipboard.EncodeOSC52("text")

// Set the tmux paste buffer via stdin. With osc52=true, tmux also forwards
// OSC 52 to the client terminal as its own output (tmux >= 3.2). This is NOT
// gated by tmux's set-clipboard option. client optionally targets -t.
err := clipboard.LoadBuffer(ctx, runner, "text", true, "client-name")

// runner implements the consumer-defined seam:
type TmuxRunner interface {
    Run(ctx context.Context, args []string, stdin io.Reader) (string, error)
}

// Native system clipboard tool (pbcopy / wl-copy / xclip / xsel / clip).
tool := clipboard.NativeTool()        // "" if none found
err := clipboard.CopyNative(ctx, "text")
```

Inside tmux, prefer `LoadBuffer(..., osc52=true, ...)` over emitting OSC 52
yourself: tmux serializes its own output, handles the client tty and the `Ms`
capability, and needs no `allow-passthrough` or DCS wrapping.

## Convenience API

```go
clipboard.Copy("text")          // all enabled targets
clipboard.CopyToTmux("text")    // tmux buffer only
clipboard.CopyToSystem("text")  // system clipboard only
clipboard.CopyWithOSC52("text") // raw OSC 52 to stderr

c := clipboard.New(
    clipboard.WithTmux(true),
    clipboard.WithSystem(false),
    clipboard.WithOSC52(true),
    clipboard.WithOutput(os.Stderr),
)
err := c.Copy("configured copy")
```

Individual writers (`NewTmuxWriter`, `NewSystemWriter`, `NewOSC52Writer`)
implement the `Writer` interface (`Write(text string) error`).

## Clipboard targets

### Tmux buffer
Uses `tmux load-buffer -`. Detects tmux via `$TMUX`. Empty text is a no-op (it
does **not** delete the buffer).

### System clipboard
Platform tools: macOS `pbcopy`; Linux `wl-copy`/`xclip`/`xsel`; Windows `clip`.

### OSC 52
Encodes text as base64 in a raw `ESC ] 52 ; c ; <base64> BEL` sequence written to
a caller-supplied `io.Writer`. The library does **not** auto-wrap in tmux DCS
passthrough — transport is the caller's choice. Empty text writes nothing.

## Environment detection

```go
available := clipboard.Available() // map: tmux / system / osc52
clipboard.IsTmuxSession()
clipboard.HasSystemClipboard()
```

## License

MIT License - see LICENSE file for details.
