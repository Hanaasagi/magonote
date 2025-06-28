package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Hanaasagi/magonote/internal/logger"
	"github.com/Hanaasagi/magonote/pkg/clipboard"
	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
)

const appName = "magonote"

var appDir = filepath.Join(xdg.StateHome, appName)
var tmpFile = filepath.Join(appDir, appName+".state")

func init() {
	err := os.MkdirAll(appDir, 0755)
	if err != nil {
		panic(fmt.Sprintf("Error creating log directory: %v\n", err))
	}

	logFilePath := filepath.Join(appDir, appName+".log")
	logger.InitLogger(logFilePath, "info")
}

// TmuxSession handles all tmux interactions with zoom state preservation
type TmuxSession struct {
	tmuxVersion string
	supportsZ   bool
}

// NewTmuxSession creates a new TmuxSession instance
func NewTmuxSession() *TmuxSession {
	// Get tmux version
	cmd := exec.Command("tmux", "-V")
	output, err := cmd.Output()
	if err != nil {
		slog.Warn("Failed to get tmux version, assuming old version", "error", err)
		return &TmuxSession{tmuxVersion: "2.0", supportsZ: false}
	}

	version := strings.TrimSpace(string(output))
	supportsZ := isVersionGTE(version, "3.1")

	slog.Info("Tmux version detected", "version", version, "supportsZ", supportsZ)
	return &TmuxSession{tmuxVersion: version, supportsZ: supportsZ}
}

// isVersionGTE checks if version is greater than or equal to target
func isVersionGTE(version, target string) bool {
	// Simple version comparison for tmux versions like "tmux 3.1"
	versionParts := regexp.MustCompile(`(\d+)\.(\d+)`).FindStringSubmatch(version)
	targetParts := regexp.MustCompile(`(\d+)\.(\d+)`).FindStringSubmatch(target)

	if len(versionParts) < 3 || len(targetParts) < 3 {
		return false
	}

	vMajor, _ := strconv.Atoi(versionParts[1])
	vMinor, _ := strconv.Atoi(versionParts[2])
	tMajor, _ := strconv.Atoi(targetParts[1])
	tMinor, _ := strconv.Atoi(targetParts[2])

	if vMajor > tMajor {
		return true
	}
	if vMajor == tMajor && vMinor >= tMinor {
		return true
	}
	return false
}

// SwapPanes swaps two panes with zoom state preservation
func (t *TmuxSession) SwapPanes(srcPane, dstPane string) error {
	args := []string{"tmux", "swap-pane", "-d", "-s", srcPane, "-t", dstPane}

	// Add -Z flag for tmux 3.1+ to preserve zoom state
	if t.supportsZ {
		args = append(args, "-Z")
	}

	slog.Info("Swapping panes", "command", args)
	cmd := exec.Command(args[0], args[1:]...)
	return cmd.Run()
}

// Execute runs a command and returns output
func (t *TmuxSession) Execute(args []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(output), "\n"), nil
}

// MagonoteRunner manages the entire magonote execution flow
type MagonoteRunner struct {
	tmux          *TmuxSession
	dir           string
	command       string
	upcaseCommand string
	multiCommand  string
	osc52         bool

	// State
	activePaneId   string
	magonotePaneId string
	signal         string
}

// NewMagonoteRunner creates a new MagonoteRunner instance
func NewMagonoteRunner(dir, command, upcaseCommand, multiCommand string, osc52 bool) *MagonoteRunner {
	sinceEpoch := time.Now().Unix()
	signal := fmt.Sprintf(appName+"-finished-%d", sinceEpoch)

	return &MagonoteRunner{
		tmux:          NewTmuxSession(),
		dir:           dir,
		command:       command,
		upcaseCommand: upcaseCommand,
		multiCommand:  multiCommand,
		osc52:         osc52,
		signal:        signal,
	}
}

// CaptureActivePane gets information about the currently active pane
func (m *MagonoteRunner) CaptureActivePane() error {
	output, err := m.tmux.Execute([]string{
		"tmux", "list-panes", "-F", "#{pane_id}:#{?pane_active,active,nope}",
	})
	if err != nil {
		return fmt.Errorf("failed to list panes: %v", err)
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) >= 2 && parts[1] == "active" {
			m.activePaneId = parts[0]
			slog.Info("Found active pane", "paneId", m.activePaneId)
			return nil
		}
	}

	return fmt.Errorf("no active pane found")
}

// CreateMagonoteWindow creates a new tmux window running magonote
func (m *MagonoteRunner) CreateMagonoteWindow() error {
	slog.Info("Creating magonote window - START")

	// Create a persistent window with a simple command (like tmux-fingers does with 'cat')
	output, err := m.tmux.Execute([]string{
		"tmux", "new-window", "-P", "-F", "#{pane_id}", "-d", "-n", "[magonote]", "cat",
	})
	if err != nil {
		return fmt.Errorf("failed to create magonote window: %v", err)
	}

	m.magonotePaneId = strings.TrimSpace(output)
	slog.Info("Created persistent magonote window", "paneId", m.magonotePaneId)

	// Now capture the original pane content and run magonote separately
	captureOutput, err := m.tmux.Execute([]string{
		"tmux", "capture-pane", "-J", "-t", m.activePaneId, "-p",
	})
	if err != nil {
		return fmt.Errorf("failed to capture pane content: %v", err)
	}

	slog.Info("Captured pane content", "lines", len(strings.Split(captureOutput, "\n")))

	// Get magonote arguments from tmux options
	args, err := m.getMagonoteArgs()
	if err != nil {
		return fmt.Errorf("failed to get magonote args: %v", err)
	}

	// Write content to temp file for magonote to process
	tempInputFile := tmpFile + ".input"
	if err := os.WriteFile(tempInputFile, []byte(captureOutput), 0644); err != nil {
		return fmt.Errorf("failed to write temp input file: %v", err)
	}

	// Run magonote on the captured content
	magonoteCmd := fmt.Sprintf("%s/build/magonote -f '%%U:%%H' -t %s %s < %s",
		m.dir, tmpFile, strings.Join(args, " "), tempInputFile)

	slog.Info("Running magonote command", "command", magonoteCmd)

	// Execute magonote in the background and signal when done
	backgroundCmd := fmt.Sprintf("(%s; tmux wait-for -S %s) &", magonoteCmd, m.signal)
	if err := exec.Command("bash", "-c", backgroundCmd).Start(); err != nil {
		os.Remove(tempInputFile)
		return fmt.Errorf("failed to start magonote: %v", err)
	}

	// Clean up temp input file
	os.Remove(tempInputFile)

	slog.Info("Magonote started in background", "signal", m.signal)
	return nil
}

// getMagonoteArgs extracts magonote arguments from tmux global options
func (m *MagonoteRunner) getMagonoteArgs() ([]string, error) {
	output, err := m.tmux.Execute([]string{"tmux", "show", "-g"})
	if err != nil {
		return nil, err
	}

	lines := strings.Split(output, "\n")
	pattern := regexp.MustCompile(`^@magonote-([\w\-0-9]+)\s+"?([^"]+)"?$`)

	var args []string
	for _, line := range lines {
		matches := pattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		name := matches[1]
		value := matches[2]

		// Handle boolean parameters
		booleanParams := []string{"reverse", "unique", "contrast"}
		for _, param := range booleanParams {
			if param == name {
				args = append(args, fmt.Sprintf("--%s", name))
				break
			}
		}

		// Handle string parameters
		stringParams := []string{
			"alphabet", "position", "fg-color", "bg-color", "hint-bg-color",
			"hint-fg-color", "select-fg-color", "select-bg-color", "multi-fg-color", "multi-bg-color",
		}
		for _, param := range stringParams {
			if param == name {
				args = append(args, fmt.Sprintf("--%s", name), fmt.Sprintf("'%s'", value))
				break
			}
		}

		// Handle regexp parameters
		if strings.HasPrefix(name, "regexp") {
			args = append(args, "--regexp", fmt.Sprintf("'%s'", strings.ReplaceAll(value, "\\\\", "\\")))
		}
	}

	return args, nil
}

// ShowMagonote swaps panes to show magonote interface
func (m *MagonoteRunner) ShowMagonote() error {
	slog.Info("ShowMagonote - START", "activePaneId", m.activePaneId, "magonotePaneId", m.magonotePaneId)

	// Wait for magonote to process the content first
	slog.Info("Waiting for magonote to process content")
	if err := m.WaitForCompletion(); err != nil {
		return fmt.Errorf("magonote processing failed: %v", err)
	}

	// Read the processed output from magonote
	processedContent, err := os.ReadFile(tmpFile)
	if err != nil {
		slog.Warn("No processed content found, using empty content", "error", err)
		processedContent = []byte("")
	}

	slog.Info("Magonote processing completed", "contentLength", len(processedContent))

	// Write the processed content to the magonote pane
	if len(processedContent) > 0 {
		if err := m.writeToPaneViaStdin(m.magonotePaneId, string(processedContent)); err != nil {
			slog.Warn("Failed to write content to magonote pane", "error", err)
		}
	}

	// Now swap the panes
	slog.Info("Swapping panes", "magonotePaneId", m.magonotePaneId, "activePaneId", m.activePaneId)
	if err := m.tmux.SwapPanes(m.magonotePaneId, m.activePaneId); err != nil {
		return fmt.Errorf("failed to swap panes: %v", err)
	}

	slog.Info("ShowMagonote - COMPLETED")
	return nil
}

// writeToPaneViaStdin writes content to a pane by sending it through tmux
func (m *MagonoteRunner) writeToPaneViaStdin(paneId, content string) error {
	// Send the content to the pane using tmux send-keys
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if _, err := m.tmux.Execute([]string{"tmux", "send-keys", "-t", paneId, line, "Enter"}); err != nil {
			return fmt.Errorf("failed to send line to pane: %v", err)
		}
	}
	return nil
}

// WaitForCompletion waits for magonote to complete
func (m *MagonoteRunner) WaitForCompletion() error {
	_, err := m.tmux.Execute([]string{"tmux", "wait-for", m.signal})
	return err
}

// CleanupMagonoteWindow swaps back and kills the magonote window
func (m *MagonoteRunner) CleanupMagonoteWindow() error {
	slog.Info("CleanupMagonoteWindow - START", "activePaneId", m.activePaneId, "magonotePaneId", m.magonotePaneId)

	// First swap back to restore original pane positions
	slog.Info("Restoring original pane layout", "swapping", m.magonotePaneId, "with", m.activePaneId)
	if err := m.tmux.SwapPanes(m.magonotePaneId, m.activePaneId); err != nil {
		slog.Warn("Failed to swap panes back", "error", err)
		// Continue with cleanup even if swap fails
	} else {
		slog.Info("Successfully swapped panes back")
	}

	// Then kill the magonote pane
	slog.Info("Cleaning up magonote pane", "paneId", m.magonotePaneId)
	_, err := m.tmux.Execute([]string{"tmux", "kill-pane", "-t", m.magonotePaneId})
	if err != nil {
		slog.Warn("Failed to kill magonote pane", "error", err)
	} else {
		slog.Info("Successfully killed magonote pane")
	}

	slog.Info("CleanupMagonoteWindow - COMPLETED")
	return err
}

// ProcessResult reads and processes the magonote result
func (m *MagonoteRunner) ProcessResult() error {
	slog.Info("ProcessResult - START")

	// At this point, the user should have interacted with the magonote interface
	// We need to wait for user input and capture the selection

	// For now, let's check if there's any selection result
	// In a real implementation, this would involve capturing user key presses
	// and processing them like tmux-fingers does

	// Read any final result from temp file
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		slog.Info("No selection result found", "error", err)
		return nil
	}

	// Clean up temp file
	defer os.Remove(tmpFile)

	result := strings.TrimSpace(string(content))
	if result == "" {
		slog.Info("No selection made")
		return nil
	}

	slog.Info("Processing user selection", "result", result)
	return m.executeCommand(result)
}

// executeCommand executes the appropriate command based on the selection
func (m *MagonoteRunner) executeCommand(result string) error {
	items := strings.Split(result, "\n")

	if len(items) > 1 {
		// Handle multiple selections
		var textParts []string
		for _, item := range items {
			parts := strings.SplitN(item, ":", 2)
			if len(parts) > 1 {
				textParts = append(textParts, parts[1])
			}
		}
		text := strings.Join(textParts, " ")

		if m.osc52 {
			if err := m.sendOSC52(text); err != nil {
				slog.Warn("Failed to send OSC52 sequence", "error", err)
			}
		}

		return m.executeFinalCommand(strings.TrimRight(text, " "), m.multiCommand)
	}

	// Handle single selection
	if len(items) == 0 || items[0] == "" {
		return nil
	}

	item := items[0]
	parts := strings.SplitN(item, ":", 2)
	if len(parts) != 2 {
		return nil
	}

	upcase := parts[0]
	text := parts[1]

	if m.osc52 {
		time.Sleep(100 * time.Millisecond) // Wait for redraw
		if err := m.sendOSC52(text); err != nil {
			slog.Warn("Failed to send OSC52 sequence", "error", err)
		}
	}

	executeCommand := m.command
	if upcase == "true" {
		executeCommand = m.upcaseCommand
	}

	return m.executeFinalCommand(strings.TrimRight(text, " "), executeCommand)
}

// sendOSC52 sends OSC52 escape sequence for clipboard integration
func (m *MagonoteRunner) sendOSC52(text string) error {
	pidBytes, err := exec.Command("tmux", "display-message", "-p", "#{pane_pid}").Output()
	if err != nil {
		return fmt.Errorf("failed to get tmux pane PID: %v", err)
	}
	pid := strings.TrimSpace(string(pidBytes))

	targetFdPath := fmt.Sprintf("/proc/%s/fd/1", pid)
	f, err := os.OpenFile(targetFdPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("failed to open tmux pane fd: %v", err)
	}
	defer f.Close()

	osc52Writer := clipboard.NewOSC52Writer(f)
	return osc52Writer.Write(text)
}

// executeFinalCommand executes the final command with the selected text
func (m *MagonoteRunner) executeFinalCommand(text, executeCommand string) error {
	finalCommand := strings.ReplaceAll(executeCommand, "{}", "${magonote}")
	cmd := exec.Command("bash", "-c", "magonote=\"$1\"; eval \"$2\"", "--", text, finalCommand)
	return cmd.Run()
}

// Run executes the complete magonote workflow
func (m *MagonoteRunner) Run() error {
	slog.Info("MagonoteRunner.Run - START")

	// Step 1: Capture active pane information
	slog.Info("Step 1: Capturing active pane information")
	if err := m.CaptureActivePane(); err != nil {
		return fmt.Errorf("failed to capture active pane: %v", err)
	}

	// Step 2: Create magonote window
	slog.Info("Step 2: Creating magonote window")
	if err := m.CreateMagonoteWindow(); err != nil {
		return fmt.Errorf("failed to create magonote window: %v", err)
	}

	// Step 3: Show magonote interface (includes waiting for completion)
	slog.Info("Step 3: Showing magonote interface")
	if err := m.ShowMagonote(); err != nil {
		return fmt.Errorf("failed to show magonote: %v", err)
	}

	// Step 4: Process the result
	slog.Info("Step 4: Processing result")
	if err := m.ProcessResult(); err != nil {
		return fmt.Errorf("failed to process result: %v", err)
	}

	// Step 5: Cleanup (swap back and kill magonote pane)
	slog.Info("Step 5: Cleaning up")
	if err := m.CleanupMagonoteWindow(); err != nil {
		slog.Warn("Failed to cleanup magonote window", "error", err)
	}

	slog.Info("MagonoteRunner.Run - COMPLETED")
	return nil
}

func parseArgs() (string, string, string, string, bool) {
	var dir, command, upcaseCommand, multiCommand string
	var osc52 bool

	rootCmd := &cobra.Command{
		Use:   "magonote-tmux",
		Short: "Tmux integration for magonote",
		Run: func(cmd *cobra.Command, args []string) {
			// Command execution logic will be handled in main
		},
	}

	rootCmd.Flags().StringVar(&dir, "dir", "", "Directory where to execute magonote")
	rootCmd.Flags().StringVar(&command, "command", "tmux set-buffer -- \"{}\" && tmux display-message \"Copied {}\"",
		"Command to execute after choosing a hint")
	rootCmd.Flags().StringVar(&upcaseCommand, "upcase-command", "tmux set-buffer -- \"{}\" && tmux paste-buffer && tmux display-message \"Copied {}\"",
		"Command to execute after choosing a hint, in upcase")
	rootCmd.Flags().StringVar(&multiCommand, "multi-command", "tmux set-buffer -- \"{}\" && tmux paste-buffer && tmux display-message \"Multi copied {}\"",
		"Command to execute after choosing multiple hints")
	rootCmd.Flags().BoolVar(&osc52, "osc52", false, "Print OSC52 copy escape sequence in addition to running the pick command")

	err := rootCmd.Execute()
	if err != nil {
		slog.Error("Failed to parse arguments", "error", err)
		os.Exit(1)
	}

	return dir, command, upcaseCommand, multiCommand, osc52
}

func main() {
	dir, command, upcaseCommand, multiCommand, osc52 := parseArgs()

	if dir == "" {
		slog.Error("Invalid tmux-magonote execution. Are you trying to execute tmux-magonote directly?")
		os.Exit(1)
	}

	slog.Info("Starting magonote-tmux", "dir", dir, "command", command, "upcaseCommand", upcaseCommand, "multiCommand", multiCommand, "osc52", osc52)

	runner := NewMagonoteRunner(dir, command, upcaseCommand, multiCommand, osc52)
	if err := runner.Run(); err != nil {
		slog.Error("Magonote execution failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Magonote execution completed successfully")
}
