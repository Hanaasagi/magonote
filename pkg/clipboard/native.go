package clipboard

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// ErrNoNativeTool is returned when no system clipboard tool is available.
var ErrNoNativeTool = errors.New("no system clipboard tool available")

// NativeTool returns the first available system clipboard tool for the
// platform, or "" if none is found on PATH.
func NativeTool() string {
	for _, tool := range getClipboardTools() {
		if _, err := exec.LookPath(tool); err == nil {
			return tool
		}
	}
	return ""
}

// CopyNative copies text to the system clipboard using the platform tool
// (pbcopy / wl-copy / xclip / xsel / clip). Empty text is a no-op. The context
// bounds the tool's runtime so a hung clipboard helper cannot block the caller.
func CopyNative(ctx context.Context, text string) error {
	if text == "" {
		return nil
	}
	tool := NativeTool()
	if tool == "" {
		return ErrNoNativeTool
	}
	cmd := exec.CommandContext(ctx, tool, nativeToolArgs(tool)...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// nativeToolArgs returns arguments that target the regular CLIPBOARD selection
// and read text from stdin. xclip and xsel default to the PRIMARY selection;
// without these flags the fallback would clobber the user's primary selection
// and leave the system clipboard unchanged. pbcopy, wl-copy and clip already
// target the system clipboard by default.
func nativeToolArgs(tool string) []string {
	switch tool {
	case "xclip":
		return []string{"-selection", "clipboard", "-i"}
	case "xsel":
		return []string{"--clipboard", "--input"}
	default:
		return nil
	}
}

// getClipboardTools returns platform-specific clipboard tools in priority order.
func getClipboardTools() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"pbcopy"}
	case "linux":
		return []string{"wl-copy", "xclip", "xsel"}
	case "windows":
		return []string{"clip"}
	default:
		return []string{}
	}
}
