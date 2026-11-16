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
	cmd := exec.CommandContext(ctx, tool)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
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
