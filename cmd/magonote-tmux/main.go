package main

import (
	"encoding/base64"
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

// Executor defines an interface for executing shell commands
type Executor interface {
	Execute(args []string) (string, error)
	LastExecuted() []string
}

// RealShell implements the Executor interface for real shell commands
type RealShell struct {
	executed []string
}

// NewRealShell creates a new RealShell instance
func NewRealShell() *RealShell {
	return &RealShell{}
}

// Execute runs a shell command and returns its output
func (s *RealShell) Execute(args []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Couldn't run command %v: %v", cmd, err) // nolint

	}

	s.executed = args
	return strings.TrimRight(string(output), "\n"), nil
}

// LastExecuted returns the last executed command
func (s *RealShell) LastExecuted() []string {
	return s.executed
}

// Swapper manages the tmux pane swapping and command execution
type Swapper struct {
	executor                 Executor
	dir                      string
	command                  string
	upcaseCommand            string
	multiCommand             string
	osc52                    bool
	activePaneId             string
	activePaneHeight         int
	activePaneScrollPosition int
	activePaneZoomed         bool
	paneId                   string
	content                  string
	signal                   string
}

// NewSwapper creates a new Swapper instance
func NewSwapper(executor Executor, dir, command, upcaseCommand, multiCommand string, osc52 bool) *Swapper {
	sinceEpoch := time.Now().Unix()
	signal := fmt.Sprintf(appName+"-finished-%d", sinceEpoch)

	return &Swapper{
		executor:      executor,
		dir:           dir,
		command:       command,
		upcaseCommand: upcaseCommand,
		multiCommand:  multiCommand,
		osc52:         osc52,
		signal:        signal,
	}
}

// CaptureActivePane captures information about the active tmux pane
func (s *Swapper) CaptureActivePane() error {
	activeCommand := []string{
		"tmux", "list-panes", "-F",
		"#{pane_id}:#{?pane_in_mode,1,0}:#{pane_height}:#{scroll_position}:#{window_zoomed_flag}:#{?pane_active,active,nope}",
	}

	output, err := s.executor.Execute(activeCommand)
	if err != nil {
		return err
	}
	lines := strings.Split(output, "\n")

	var activePaneInfo []string
	for _, line := range lines {
		chunks := strings.Split(line, ":")
		if len(chunks) > 5 && chunks[5] == "active" {
			activePaneInfo = chunks
			break
		}
	}

	if len(activePaneInfo) == 0 {
		return fmt.Errorf("unable to find active pane")
	}

	s.activePaneId = activePaneInfo[0]

	paneHeight, err := strconv.Atoi(activePaneInfo[2])
	if err != nil {
		return fmt.Errorf("unable to retrieve pane height: %v", err)
	}
	s.activePaneHeight = paneHeight

	if activePaneInfo[1] == "1" {
		scrollPosition, err := strconv.Atoi(activePaneInfo[3])
		if err != nil {
			return fmt.Errorf("unable to retrieve pane scroll: %v", err)
		}
		s.activePaneScrollPosition = scrollPosition
	}

	s.activePaneZoomed = activePaneInfo[4] == "1"
	return nil
}

// ExecuteMagonote executes the magonote command in a new tmux window
func (s *Swapper) ExecuteMagonote() error {
	optionsCommand := []string{"tmux", "show", "-g"}
	options, err := s.executor.Execute(optionsCommand)
	if err != nil {
		return err
	}
	lines := strings.Split(options, "\n")

	pattern := regexp.MustCompile(`^@magonote-([\w\-0-9]+)\s+"?([^"]+)"?$`)

	var args []string
	for _, line := range lines {
		matches := pattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		name := matches[1]
		value := matches[2]

		booleanParams := []string{"reverse", "unique", "contrast"}
		for _, param := range booleanParams {
			if param == name {
				args = append(args, fmt.Sprintf("--%s", name))
				break
			}
		}

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

		if strings.HasPrefix(name, "regexp") {
			args = append(args, "--regexp", fmt.Sprintf("'%s'", strings.ReplaceAll(value, "\\\\", "\\")))
		}
	}

	scrollParams := ""
	if s.activePaneScrollPosition != 0 {
		scrollParams = fmt.Sprintf(" -S %d -E %d", s.activePaneScrollPosition, s.activePaneHeight-s.activePaneScrollPosition-1)
	}

	zoomCommand := ""
	if s.activePaneZoomed {
		zoomCommand = fmt.Sprintf("tmux resize-pane -t %s -Z;", s.activePaneId)
	}

	paneCommand := fmt.Sprintf(
		"tmux capture-pane -J -t %s -p%s | tail -n %d | %s/build/magonote -f '%%U:%%H' -t %s %s; tmux swap-pane -t %s; %s tmux wait-for -S %s",
		s.activePaneId,
		scrollParams,
		s.activePaneHeight,
		s.dir,
		tmpFile,
		strings.Join(args, " "),
		s.activePaneId,
		zoomCommand,
		s.signal,
	)

	command := []string{
		"tmux", "new-window", "-P", "-F", "#{pane_id}", "-d", "-n", "[magonote]", paneCommand,
	}

	s.paneId, err = s.executor.Execute(command)
	if err != nil {
		return err
	}
	return nil
}

// SwapPanes swaps the active pane with the magonote pane
func (s *Swapper) SwapPanes() error {
	swapCommand := []string{
		"tmux", "swap-pane", "-d", "-s", s.activePaneId, "-t", s.paneId,
	}

	var filteredCommand []string
	for _, arg := range swapCommand {
		if arg != "" {
			filteredCommand = append(filteredCommand, arg)
		}
	}

	_, err := s.executor.Execute(filteredCommand)
	if err != nil {
		return err
	}
	return nil
}

// ResizePane resizes the pane to match the active pane's zoom state
func (s *Swapper) ResizePane() error {
	if !s.activePaneZoomed {
		return nil
	}

	resizeCommand := []string{"tmux", "resize-pane", "-t", s.paneId, "-Z"}

	var filteredCommand []string
	for _, arg := range resizeCommand {
		if arg != "" {
			filteredCommand = append(filteredCommand, arg)
		}
	}

	_, err := s.executor.Execute(filteredCommand)
	if err != nil {
		return err
	}
	return nil
}

// Wait waits for the magonote process to complete
func (s *Swapper) Wait() error {
	waitCommand := []string{"tmux", "wait-for", s.signal}
	_, err := s.executor.Execute(waitCommand)
	if err != nil {
		return err
	}
	return nil
}

// RetrieveContent reads the content from the temporary file
func (s *Swapper) RetrieveContent() error {
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		return err
	}
	s.content = string(content)
	return nil
}

// DestroyContent removes the temporary file
func (s *Swapper) DestroyContent() error {
	err := os.Remove(tmpFile)
	return err
}

func (s *Swapper) SendOSC52() {
	// TODO:
}

// ExecuteCommand executes the appropriate command based on the selected content
func (s *Swapper) ExecuteCommand() error {
	items := strings.Split(s.content, "\n")

	if len(items) > 1 {
		var textParts []string
		for _, item := range items {
			parts := strings.SplitN(item, ":", 2)
			if len(parts) > 1 {
				textParts = append(textParts, parts[1])
			}
		}
		text := strings.Join(textParts, " ")
		err := s.ExecuteFinalCommand(strings.TrimRight(text, " "), s.multiCommand)
		return err
	}

	// Only one item
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

	if s.osc52 {
		base64Text := base64.StdEncoding.EncodeToString([]byte(text))
		oscSeq := fmt.Sprintf("\x1b]52;0;%s\x07", base64Text)
		tmuxSeq := fmt.Sprintf("\x1bPtmux;%s\x1b\\", strings.ReplaceAll(oscSeq, "\x1b", "\x1b\x1b"))

		// Wait a bit for the redraw to finish
		time.Sleep(100 * time.Millisecond)

		_, err := os.Stdout.Write([]byte(tmuxSeq))
		if err != nil {
			return err
		}
		err = os.Stdout.Sync()
		if err != nil {
			return err
		}
	}

	executeCommand := s.command
	if upcase == "true" {
		executeCommand = s.upcaseCommand
	}

	err := s.ExecuteFinalCommand(strings.TrimRight(text, " "), executeCommand)
	if err != nil {
		return err
	}
	return nil
}

// ExecuteFinalCommand executes the final command with the selected text
func (s *Swapper) ExecuteFinalCommand(text, executeCommand string) error {
	finalCommand := strings.ReplaceAll(executeCommand, "{}", "${magonote}")
	retrieveCommand := []string{
		"bash", "-c", "magonote=\"$1\"; eval \"$2\"", "--", text, finalCommand,
	}

	_, err := s.executor.Execute(retrieveCommand)
	if err != nil {
		return err
	}
	return nil
}

func appArgs() (string, string, string, string, bool) {
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
	dir, command, upcaseCommand, multiCommand, osc52 := appArgs()

	if dir == "" {
		panic("Invalid tmux-magonote execution. Are you trying to execute tmux-magonote directly?")
	}
	slog.Info("Running tmux-magonote", "dir", dir, "command", command, "upcaseCommand", upcaseCommand, "multiCommand", multiCommand, "osc52", osc52)

	executor := NewRealShell()
	swapper := NewSwapper(executor, dir, command, upcaseCommand, multiCommand, osc52)

	mustDo := func(msg string, fn func() error) {
		if err := fn(); err != nil {
			slog.Error(msg, "error", err)
			os.Exit(1)
		}
	}

	mustDo("Failed to capture active pane", swapper.CaptureActivePane)
	mustDo("Failed to execute magonote", swapper.ExecuteMagonote)
	mustDo("Failed to swap panes", swapper.SwapPanes)
	mustDo("Failed to resize pane", swapper.ResizePane)
	mustDo("Failed to wait for magonote", swapper.Wait)
	mustDo("Failed to retrieve content", swapper.RetrieveContent)
	mustDo("Failed to destroy content", swapper.DestroyContent)
	mustDo("Failed to execute command", swapper.ExecuteCommand)
}
