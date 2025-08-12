package main

import (
	"bytes"
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

var (
	appDir  = filepath.Join(xdg.StateHome, appName)
	tmpFile = filepath.Join(appDir, appName+".state")
)

func init() {
	if err := os.MkdirAll(appDir, 0755); err != nil {
		panic(fmt.Sprintf("Error creating log directory: %v\n", err))
	}

	logFilePath := filepath.Join(appDir, appName+".log")

	logLevel := os.Getenv("MAGONOTE_LOG")
	if logLevel == "" {
		logLevel = "info"
	}

	logger.InitLogger(logFilePath, logLevel)
}

// PaneInfo holds comprehensive information about a tmux pane including scroll state
type PaneInfo struct {
	ID             string // Pane ID (e.g., "%1")
	Height         int    // Pane height in lines
	ScrollPosition int    // Current scroll position (only valid when InMode is true)
	InMode         bool   // Whether pane is in copy/scroll mode
	Zoomed         bool   // Whether the pane is zoomed
}

// HasScrollData returns true if the pane has valid scroll information
func (p *PaneInfo) HasScrollData() bool {
	return p.InMode && p.ScrollPosition >= 0
}

// Config holds all configuration for magonote execution
type Config struct {
	Dir           string
	Command       string
	UpcaseCommand string
	MultiCommand  string
	OSC52         bool
}

// Magonote orchestrates the complete tmux-magonote workflow
type Magonote struct {
	config Config
	signal string

	// Runtime state
	activePaneInfo *PaneInfo
	magonotePaneID string
	ps1Pattern     string // Detected PS1 pattern
}

// New creates a new Magonote instance with the given configuration
func New(config Config) *Magonote {
	sinceEpoch := time.Now().Unix()
	signal := fmt.Sprintf("%s-finished-%d", appName, sinceEpoch)

	return &Magonote{
		config: config,
		signal: signal,
	}
}

// Run executes the complete magonote workflow
func (m *Magonote) Run() error {
	slog.Debug("Starting magonote workflow")

	if err := m.captureActivePane(); err != nil {
		return fmt.Errorf("capturing active pane: %w", err)
	}

	if err := m.detectPS1Pattern(); err != nil {
		slog.Warn("Failed to detect PS1 pattern", "error", err)
		// Continue without PS1 filtering
	}

	if err := m.createMagonoteWindow(); err != nil {
		return fmt.Errorf("creating magonote window: %w", err)
	}

	if err := m.showMagonoteInterface(); err != nil {
		return fmt.Errorf("showing magonote interface: %w", err)
	}

	if err := m.waitForUserInteraction(); err != nil {
		return fmt.Errorf("waiting for user interaction: %w", err)
	}

	if err := m.processUserSelection(); err != nil {
		return fmt.Errorf("processing user selection: %w", err)
	}

	if err := m.cleanup(); err != nil {
		slog.Warn("Cleanup failed", "error", err)
	}

	slog.Debug("Magonote workflow completed successfully")
	return nil
}

// captureActivePane identifies and stores comprehensive information about the currently active pane
func (m *Magonote) captureActivePane() error {
	// Format: #{pane_id}:#{?pane_in_mode,1,0}:#{pane_height}:#{scroll_position}:#{window_zoomed_flag}:#{?pane_active,active,nope}
	output, err := m.tmuxCommand("list-panes", "-F",
		"#{pane_id}:#{?pane_in_mode,1,0}:#{pane_height}:#{scroll_position}:#{window_zoomed_flag}:#{?pane_active,active,nope}")
	if err != nil {
		return fmt.Errorf("listing panes: %w", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) < 6 {
			slog.Warn("Unexpected pane format", "line", line, "parts", len(parts))
			continue
		}

		// Check if this is the active pane
		if parts[5] != "active" {
			continue
		}

		paneInfo, err := m.parsePaneInfo(parts)
		if err != nil {
			slog.Warn("Failed to parse pane info", "error", err, "line", line)
			continue
		}

		m.activePaneInfo = paneInfo
		slog.Debug("Captured active pane", "paneID", m.activePaneInfo.ID,
			"height", m.activePaneInfo.Height, "inMode", m.activePaneInfo.InMode,
			"scrollPosition", m.activePaneInfo.ScrollPosition, "zoomed", m.activePaneInfo.Zoomed)
		return nil
	}

	return fmt.Errorf("no active pane found")
}

// parsePaneInfo parses pane information from tmux list-panes output
func (m *Magonote) parsePaneInfo(parts []string) (*PaneInfo, error) {
	if len(parts) < 6 {
		return nil, fmt.Errorf("insufficient pane data parts: expected 6, got %d", len(parts))
	}

	paneInfo := &PaneInfo{
		ID: parts[0],
	}

	// Parse pane_in_mode (1 = in mode, 0 = normal)
	if inMode, err := strconv.Atoi(parts[1]); err != nil {
		return nil, fmt.Errorf("parsing pane_in_mode: %w", err)
	} else {
		paneInfo.InMode = (inMode == 1)
	}

	// Parse pane height
	if height, err := strconv.Atoi(parts[2]); err != nil {
		return nil, fmt.Errorf("parsing pane_height: %w", err)
	} else {
		paneInfo.Height = height
	}

	// Parse scroll position (only meaningful when in mode)
	if paneInfo.InMode {
		if scrollPos, err := strconv.Atoi(parts[3]); err != nil {
			return nil, fmt.Errorf("parsing scroll_position: %w", err)
		} else {
			paneInfo.ScrollPosition = scrollPos
		}
	} else {
		paneInfo.ScrollPosition = 0
	}

	// Parse zoomed flag
	if zoomed, err := strconv.Atoi(parts[4]); err != nil {
		return nil, fmt.Errorf("parsing window_zoomed_flag: %w", err)
	} else {
		paneInfo.Zoomed = (zoomed == 1)
	}

	return paneInfo, nil
}

// detectPS1Pattern detects the current shell's PS1 pattern
func (m *Magonote) detectPS1Pattern() error {
	slog.Debug("Detecting PS1 pattern")

	// Get current working directory from the active pane
	workingDir, err := m.getActivePaneWorkingDir()
	if err != nil {
		slog.Warn("Failed to get working directory", "error", err)
		workingDir = "" // Use current directory as fallback
	}

	// Get shell prompt
	ps1Pattern, err := m.getShellPrompt(workingDir)
	if err != nil {
		return fmt.Errorf("getting shell prompt: %w", err)
	}

	if ps1Pattern == "" {
		slog.Debug("No PS1 pattern detected")
		return nil
	}

	m.ps1Pattern = ps1Pattern
	slog.Debug("Detected PS1 pattern", "pattern", ps1Pattern)
	return nil
}

// getActivePaneWorkingDir gets the working directory of the active pane
func (m *Magonote) getActivePaneWorkingDir() (string, error) {
	if m.activePaneInfo == nil {
		return "", fmt.Errorf("no active pane info available")
	}

	// Get the current path using tmux's pane_current_path
	output, err := m.tmuxCommand("display-message", "-t", m.activePaneInfo.ID, "-p", "#{pane_current_path}")
	if err != nil {
		return "", fmt.Errorf("getting pane current path: %w", err)
	}

	workingDir := strings.TrimSpace(output)
	slog.Debug("Active pane working directory", "dir", workingDir)
	return workingDir, nil
}

// getShellPrompt gets the shell prompt for the given directory
func (m *Magonote) getShellPrompt(workingDir string) (string, error) {
	// Detect shell from SHELL environment variable
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "zsh" // Default to zsh
	} else {
		shellPath = filepath.Base(shellPath)
	}

	var cmd *exec.Cmd
	var cmdStr string

	switch shellPath {
	case "zsh":
		// Use zsh -i -c with print -P for prompt expansion in the specified directory
		if workingDir != "" {
			cmdStr = fmt.Sprintf(`cd "%s" && echo "$PS1"`, workingDir)
		} else {
			cmdStr = `echo "$PS1"`
		}
		cmd = exec.Command("zsh", "-i", "-c", cmdStr)
	case "bash":
		// Use bash -i -c to get expanded PS1 in the specified directory
		if workingDir != "" {
			cmdStr = fmt.Sprintf(`cd "%s" && echo "${PS1@P}"`, workingDir)
		} else {
			cmdStr = `echo "${PS1@P}"`
		}
		cmd = exec.Command("bash", "-i", "-c", cmdStr)
	default:
		// For unknown shells, try zsh approach as our target is primarily zsh
		slog.Warn("Unknown shell detected, trying zsh approach", "shell", shellPath)
		if workingDir != "" {
			cmdStr = fmt.Sprintf(`cd "%s" && print -P "$PS1"`, workingDir)
		} else {
			cmdStr = `print -P "$PS1"`
		}
		cmd = exec.Command("zsh", "-i", "-c", cmdStr)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()
	if err != nil {
		slog.Warn("Failed to get shell prompt", "error", err, "shell", shellPath, "workingDir", workingDir, "stderr", errOut.String())
		return "", nil // Return empty string instead of error to avoid breaking the workflow
	}

	prompt := strings.TrimSpace(out.String())
	slog.Debug("Retrieved shell prompt", "prompt", prompt, "shell", shellPath, "workingDir", workingDir)
	return prompt, nil
}

// buildScrollParams generates tmux capture-pane scroll parameters based on pane state
func (m *Magonote) buildScrollParams() string {
	if m.activePaneInfo == nil || !m.activePaneInfo.HasScrollData() {
		return ""
	}

	// Following tmux-thumbs logic: -S -scroll_position -E pane_height-scroll_position-1
	startLine := -m.activePaneInfo.ScrollPosition
	endLine := m.activePaneInfo.Height - m.activePaneInfo.ScrollPosition - 1

	return fmt.Sprintf(" -S %d -E %d", startLine, endLine)
}

// buildCaptureCommand generates the tmux capture-pane command with proper scroll handling
func (m *Magonote) buildCaptureCommand() string {
	scrollParams := m.buildScrollParams()

	// Base capture command with ANSI escape sequences and join lines
	captureCmd := fmt.Sprintf("tmux capture-pane -J -t %s -p -e",
		m.activePaneInfo.ID)

	// Append scroll params only when present to avoid trailing spaces
	if scrollParams != "" {
		captureCmd += scrollParams
	}

	// Add tail to limit output height when we have scroll data
	if m.activePaneInfo.HasScrollData() {
		captureCmd += fmt.Sprintf(" | tail -n %d", m.activePaneInfo.Height)
	}

	return captureCmd
}

// createMagonoteWindow creates a new tmux window running the magonote command
func (m *Magonote) createMagonoteWindow() error {
	slog.Debug("Creating magonote window")

	args, err := m.buildMagonoteArgs()
	if err != nil {
		return fmt.Errorf("building magonote arguments: %w", err)
	}

	// Build the command that will keep the pane alive after magonote completes
	captureCmd := m.buildCaptureCommand()
	command := fmt.Sprintf(
		"%s | %s/magonote -f '%%U:%%H' -t %s %s; tmux wait-for -S %s; sleep infinity",
		captureCmd,
		m.config.Dir,
		tmpFile,
		strings.Join(args, " "),
		m.signal,
	)

	slog.Debug("Executing magonote command", "command", command)

	output, err := m.tmuxCommand("new-window", "-P", "-F", "#{pane_id}", "-d", "-n", "[magonote]", command)
	if err != nil {
		return fmt.Errorf("creating new window: %w", err)
	}

	m.magonotePaneID = strings.TrimSpace(output)
	slog.Debug("Created magonote window", "paneID", m.magonotePaneID)
	return nil
}

// buildMagonoteArgs extracts and formats magonote arguments from tmux options
func (m *Magonote) buildMagonoteArgs() ([]string, error) {
	output, err := m.tmuxCommand("show", "-g")
	if err != nil {
		return nil, fmt.Errorf("showing global options: %w", err)
	}

	pattern := regexp.MustCompile(`^@magonote-([\w\-0-9]+)\s+"?([^"]+)"?$`)
	var args []string

	for _, line := range strings.Split(output, "\n") {
		matches := pattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		name, value := matches[1], matches[2]

		switch {
		case m.isBooleanParam(name):
			args = append(args, fmt.Sprintf("--%s", name))
		case m.isStringParam(name):
			args = append(args, fmt.Sprintf("--%s", name), fmt.Sprintf("'%s'", value))
		case strings.HasPrefix(name, "regexp"):
			args = append(args, "--regexp", fmt.Sprintf("'%s'", strings.ReplaceAll(value, "\\\\", "\\")))
		}
	}

	// Add PS1 pattern if detected
	if m.ps1Pattern != "" {
		args = append(args, "--ps1-pattern", fmt.Sprintf("'%s'", m.ps1Pattern))
		slog.Debug("Added PS1 pattern to magonote args", "pattern", m.ps1Pattern)
	}

	return args, nil
}

// isBooleanParam checks if the parameter is a boolean type
func (m *Magonote) isBooleanParam(name string) bool {
	booleanParams := []string{"reverse", "unique", "contrast"}
	for _, param := range booleanParams {
		if param == name {
			return true
		}
	}
	return false
}

// isStringParam checks if the parameter is a string type
func (m *Magonote) isStringParam(name string) bool {
	stringParams := []string{
		"alphabet", "position", "fg-color", "bg-color", "hint-bg-color",
		"hint-fg-color", "select-fg-color", "select-bg-color", "multi-fg-color", "multi-bg-color",
	}
	for _, param := range stringParams {
		if param == name {
			return true
		}
	}
	return false
}

// showMagonoteInterface swaps panes to display the magonote interface
func (m *Magonote) showMagonoteInterface() error {
	slog.Debug("Showing magonote interface", "from", m.magonotePaneID, "to", m.activePaneInfo.ID)

	if err := m.swapPanes(m.magonotePaneID, m.activePaneInfo.ID); err != nil {
		return fmt.Errorf("swapping panes: %w", err)
	}

	slog.Debug("Magonote interface displayed successfully")
	return nil
}

// waitForUserInteraction waits for the user to complete their interaction with magonote
func (m *Magonote) waitForUserInteraction() error {
	slog.Debug("Waiting for user interaction", "signal", m.signal)

	if _, err := m.tmuxCommand("wait-for", m.signal); err != nil {
		return fmt.Errorf("waiting for signal: %w", err)
	}

	slog.Debug("User interaction completed")
	m.verifyPaneStates()
	return nil
}

// verifyPaneStates checks the current state of both panes for debugging
func (m *Magonote) verifyPaneStates() {
	if err := m.checkPaneExists(m.activePaneInfo.ID, "active"); err != nil {
		slog.Warn("Active pane verification failed", "error", err)
	}

	if err := m.checkPaneExists(m.magonotePaneID, "magonote"); err != nil {
		slog.Warn("Magonote pane verification failed", "error", err)
	}
}

// checkPaneExists verifies that a specific pane still exists
func (m *Magonote) checkPaneExists(paneID, description string) error {
	output, err := m.tmuxCommand("list-panes", "-a", "-F", "#{pane_id}")
	if err != nil {
		return fmt.Errorf("listing all panes: %w", err)
	}

	for _, pane := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(pane) == paneID {
			slog.Debug("Pane exists", "description", description, "paneID", paneID)
			return nil
		}
	}

	return fmt.Errorf("pane %s (%s) not found", description, paneID)
}

// processUserSelection reads and processes the user's selection from magonote
func (m *Magonote) processUserSelection() error {
	slog.Debug("Processing user selection")

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		slog.Info("No selection found", "error", err)
		return nil
	}
	defer os.Remove(tmpFile) // nolint: errcheck

	result := strings.TrimSpace(string(content))
	if result == "" {
		slog.Info("No selection made by user")
		return nil
	}

	slog.Info("User made selection", "result", result)
	return m.executeSelectionCommand(result)
}

// executeSelectionCommand executes the appropriate command based on the user's selection
func (m *Magonote) executeSelectionCommand(result string) error {
	items := strings.Split(result, "\n")

	if len(items) > 1 {
		return m.handleMultipleSelection(items)
	}

	if len(items) == 0 || items[0] == "" {
		return nil
	}

	return m.handleSingleSelection(items[0])
}

// handleMultipleSelection processes multiple selected items
func (m *Magonote) handleMultipleSelection(items []string) error {
	var textParts []string
	for _, item := range items {
		if parts := strings.SplitN(item, ":", 2); len(parts) > 1 {
			textParts = append(textParts, parts[1])
		}
	}

	text := strings.Join(textParts, " ")

	if m.config.OSC52 {
		if err := m.sendOSC52Sequence(text); err != nil {
			slog.Warn("Failed to send OSC52 sequence", "error", err)
		}
	}

	return m.executeFinalCommand(strings.TrimRight(text, " "), m.config.MultiCommand)
}

// handleSingleSelection processes a single selected item
func (m *Magonote) handleSingleSelection(item string) error {
	parts := strings.SplitN(item, ":", 2)
	if len(parts) != 2 {
		return nil
	}

	upcase, text := parts[0], parts[1]

	if m.config.OSC52 {
		time.Sleep(100 * time.Millisecond) // Wait for redraw
		if err := m.sendOSC52Sequence(text); err != nil {
			slog.Warn("Failed to send OSC52 sequence", "error", err)
		}
	}

	command := m.config.Command
	if upcase == "true" {
		command = m.config.UpcaseCommand
	}

	return m.executeFinalCommand(strings.TrimRight(text, " "), command)
}

// sendOSC52Sequence sends an OSC52 escape sequence for clipboard integration
func (m *Magonote) sendOSC52Sequence(text string) error {
	pidOutput, err := m.tmuxCommand("display-message", "-p", "#{pane_pid}")
	if err != nil {
		return fmt.Errorf("getting tmux pane PID: %w", err)
	}

	pid := strings.TrimSpace(pidOutput)
	targetFdPath := fmt.Sprintf("/proc/%s/fd/1", pid)

	file, err := os.OpenFile(targetFdPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("opening tmux pane fd: %w", err)
	}
	defer file.Close() // nolint: errcheck

	osc52Writer := clipboard.NewOSC52Writer(file)
	return osc52Writer.Write(text)
}

// executeFinalCommand executes the final command with the selected text
func (m *Magonote) executeFinalCommand(text, command string) error {
	finalCommand := strings.ReplaceAll(command, "{}", "${magonote}")
	slog.Info("Executing final command", "text", text, "command", finalCommand)
	cmd := exec.Command("bash", "-c", "magonote=\"$1\"; eval \"$2\"", "--", text, finalCommand)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		slog.Error("Final command execution failed", "error", err, "stderr", stderr.String(), "stdout", stdout.String())
	}

	return err
}

// cleanup restores the original pane layout and removes the magonote window
func (m *Magonote) cleanup() error {
	slog.Debug("Starting cleanup", "activePaneID", m.activePaneInfo.ID, "magonotePaneID", m.magonotePaneID)

	activeExists := m.checkPaneExists(m.activePaneInfo.ID, "active") == nil
	magonoteExists := m.checkPaneExists(m.magonotePaneID, "magonote") == nil

	slog.Debug("Pane existence status", "activeExists", activeExists, "magonoteExists", magonoteExists)

	if !activeExists {
		return fmt.Errorf("active pane %s no longer exists", m.activePaneInfo.ID)
	}

	if !magonoteExists {
		slog.Warn("Magonote pane no longer exists, skipping restoration", "paneID", m.magonotePaneID)
		return nil
	}

	// Restore original pane layout
	slog.Debug("Restoring original pane layout")
	if err := m.swapPanes(m.magonotePaneID, m.activePaneInfo.ID); err != nil {
		slog.Warn("Failed to restore pane layout", "error", err)
	} else {
		slog.Debug("Successfully restored pane layout")
	}

	// Remove magonote pane
	slog.Debug("Removing magonote pane", "paneID", m.magonotePaneID)
	if err := m.killPane(m.magonotePaneID); err != nil {
		slog.Warn("Failed to kill magonote pane", "error", err)
		return err
	}

	slog.Debug("Cleanup completed successfully")
	return nil
}

// swapPanes swaps two tmux panes with zoom state preservation
func (m *Magonote) swapPanes(srcPane, dstPane string) error {
	args := []string{"swap-pane", "-d", "-s", srcPane, "-t", dstPane, "-Z"}
	slog.Debug("Swapping panes", "src", srcPane, "dst", dstPane)

	_, err := m.tmuxCommand(args...)
	return err
}

// killPane terminates a specific tmux pane
func (m *Magonote) killPane(paneID string) error {
	_, err := m.tmuxCommand("kill-pane", "-t", paneID)
	if err == nil {
		slog.Debug("Successfully terminated pane", "paneID", paneID)
	}
	return err
}

// tmuxCommand executes a tmux command and returns its output
func (m *Magonote) tmuxCommand(args ...string) (string, error) {
	fullArgs := append([]string{"tmux"}, args...)
	cmd := exec.Command(fullArgs[0], fullArgs[1:]...)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tmux command failed: %w", err)
	}

	return strings.TrimRight(string(output), "\n"), nil
}

// parseCommandLineArgs parses command line arguments and returns configuration
func parseCommandLineArgs() Config {
	var config Config

	rootCmd := &cobra.Command{
		Use:   "magonote-tmux",
		Short: "Tmux integration for magonote",
		Run: func(cmd *cobra.Command, args []string) {
			// Command execution is handled in main
		},
	}

	rootCmd.Flags().StringVar(&config.Dir, "dir", "", "Directory where to execute magonote")
	rootCmd.Flags().StringVar(&config.Command, "command",
		"tmux set-buffer -- \"{}\" && tmux display-message \"Copied {}\"",
		"Command to execute after choosing a hint")
	rootCmd.Flags().StringVar(&config.UpcaseCommand, "upcase-command",
		"tmux set-buffer -- \"{}\" && tmux paste-buffer && tmux display-message \"Copied {}\"",
		"Command to execute after choosing a hint, in upcase")
	rootCmd.Flags().StringVar(&config.MultiCommand, "multi-command",
		"tmux set-buffer -- \"{}\" && tmux paste-buffer && tmux display-message \"Multi copied {}\"",
		"Command to execute after choosing multiple hints")
	rootCmd.Flags().BoolVar(&config.OSC52, "osc52", false,
		"Print OSC52 copy escape sequence in addition to running the pick command")

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Failed to parse command line arguments", "error", err)
		os.Exit(1)
	}

	return config
}

func searchMagonoteBinaryDirectory() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	execDir := filepath.Dir(execPath)

	return execDir, nil
}

func main() {
	config := parseCommandLineArgs()

	if config.Dir == "" {
		slog.Error("Missing --dir flag, trying to determine magonote binary directory")
		execDir, err := searchMagonoteBinaryDirectory()
		if err != nil {
			slog.Error("Failed to determine magonote binary directory", "error", err)
			os.Exit(1)
		}
		slog.Info("Determined magonote binary directory", "dir", execDir)
		config.Dir = execDir
	}

	slog.Info("Starting magonote-tmux",
		"dir", config.Dir,
		"command", config.Command,
		"upcaseCommand", config.UpcaseCommand,
		"multiCommand", config.MultiCommand,
		"osc52", config.OSC52)

	magonote := New(config)
	if err := magonote.Run(); err != nil {
		slog.Error("Magonote execution failed", "error", err)
		os.Exit(1)
	}
}
