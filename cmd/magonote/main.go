package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/Hanaasagi/magonote/cmd"
	"github.com/Hanaasagi/magonote/internal"
	"github.com/Hanaasagi/magonote/internal/logger"
	"github.com/adrg/xdg"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const (
	appName       = "magonote"
	defaultSize   = 4096
	defaultEditor = "vi"
)

var (
	Version     = "0.1.0"
	CommitSha   = "unknown"
	FullVersion = Version + "-" + CommitSha
)

var appDir = filepath.Join(xdg.StateHome, appName)

// Arguemnt holds application configuration
type Arguemnt struct {
	alphabet       string
	format         string
	colors         ColorConfig
	position       string
	regexpPatterns []string
	flags          FlagConfig
	target         string
	inputFile      string
	showVersion    bool
}

func init() {
	// Initialize logging
	if err := os.MkdirAll(appDir, 0755); err != nil {
		panic(fmt.Sprintf("Error creating log directory: %v", err))
	}

	logFilePath := filepath.Join(appDir, appName+".log")
	logger.InitLogger(logFilePath, "info")

	// Initialize crash reporting
	crashFilePath := filepath.Join(appDir, "crash")
	if f, err := os.Create(crashFilePath); err == nil {
		_ = debug.SetCrashOutput(f, debug.CrashOptions{})
	}
}

// readInput reads input from file or stdin with buffering
func readInput(inputFile string) ([]string, error) {
	var reader io.Reader
	var closer io.Closer

	if inputFile != "" {
		file, err := os.Open(inputFile)
		if err != nil {
			return nil, fmt.Errorf("opening input file: %w", err)
		}
		reader = file
		closer = file
	} else {
		reader = os.Stdin
	}

	defer func() {
		if closer != nil {
			closer.Close() // nolint: errcheck
		}
	}()

	bufferedReader := bufio.NewReaderSize(reader, defaultSize)
	var lines []string

	for {
		line, err := bufferedReader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("reading input: %w", err)
		}

		if line != "" {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}

		if err == io.EOF {
			break
		}
	}

	return lines, nil
}

// writeOutput writes output to target file or stdout with buffering
func writeOutput(target, content string) error {
	if target == "" {
		fmt.Print(content)
		return nil
	}

	file, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("creating target file: %w", err)
	}
	defer file.Close() // nolint: errcheck

	writer := bufio.NewWriterSize(file, defaultSize)
	defer writer.Flush() // nolint: errcheck

	if _, err := writer.WriteString(content); err != nil {
		return fmt.Errorf("writing to target file: %w", err)
	}

	return nil
}

// openFileWithEditor opens the specified file with the editor
func openFileWithEditor(filePath string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = defaultEditor
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %w", err)
	}

	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// processResults processes selected items and returns formatted output
func processResults(selected []internal.ChosenMatch, format string) (string, error) {
	if len(selected) == 0 {
		return "", nil
	}

	results := make([]string, 0, len(selected))

	for _, item := range selected {
		if item.ShouldOpenFile {
			slog.Info("Opening file with editor", "file", item.Text, "editor", os.Getenv("EDITOR"))
			if err := openFileWithEditor(item.Text); err != nil {
				return "", fmt.Errorf("opening file with editor: %w", err)
			}
			os.Exit(0)
		}

		result := strings.ReplaceAll(format, "%H", item.Text)
		upcase := "false"
		if item.Uppercase {
			upcase = "true"
		}
		result = strings.ReplaceAll(result, "%U", upcase)
		results = append(results, result)
	}

	return strings.Join(results, "\n"), nil
}

// loadConfig loads and merges configuration from multiple sources
func loadConfig(configPath string) (*Config, error) {
	var actualConfigPath string

	if configPath != "" {
		actualConfigPath = configPath
	} else {
		// Use XDG config directory
		actualConfigPath = filepath.Join(xdg.ConfigHome, appName, "config.toml")
	}

	config, err := LoadConfigFromFile(actualConfigPath)
	if err != nil {
		return nil, fmt.Errorf("loading config from %s: %w", actualConfigPath, err)
	}

	return config, nil
}

// applyCliOverrides applies CLI arguments to override config values
func applyCliOverrides(config *Config, argument *Arguemnt) {
	if argument.alphabet != "qwerty" {
		config.Alphabet = argument.alphabet
	}
	if argument.format != "%H" {
		config.Format = argument.format
	}
	if argument.position != "left" {
		config.Position = argument.position
	}
	if len(argument.regexpPatterns) > 0 {
		config.RegexpPatterns = argument.regexpPatterns
	}

	// Colors - apply if different from defaults
	if argument.colors.Foreground != "green" {
		config.Colors.Foreground = argument.colors.Foreground
	}
	if argument.colors.Background != "black" {
		config.Colors.Background = argument.colors.Background
	}
	if argument.colors.HintForeground != "yellow" {
		config.Colors.HintForeground = argument.colors.HintForeground
	}
	if argument.colors.HintBackground != "black" {
		config.Colors.HintBackground = argument.colors.HintBackground
	}
	if argument.colors.MultiForeground != "yellow" {
		config.Colors.MultiForeground = argument.colors.MultiForeground
	}
	if argument.colors.MultiBackground != "black" {
		config.Colors.MultiBackground = argument.colors.MultiBackground
	}
	if argument.colors.SelectForeground != "blue" {
		config.Colors.SelectForeground = argument.colors.SelectForeground
	}
	if argument.colors.SelectBackground != "black" {
		config.Colors.SelectBackground = argument.colors.SelectBackground
	}

	// Flags - always apply from CLI since they might override config
	config.Flags.Multi = argument.flags.Multi
	config.Flags.Reverse = argument.flags.Reverse
	config.Flags.UniqueLevel = argument.flags.UniqueLevel
	config.Flags.Contrast = argument.flags.Contrast

}

// runApp runs the main application logic
func runApp(config *Config, inputFile, target string) error {

	lines, err := readInput(inputFile)
	if err != nil {
		return err
	}

	state := internal.NewState(lines, config.Alphabet, config.RegexpPatterns)
	viewbox := internal.NewView(
		state,
		config.Flags.Multi,
		config.Flags.Reverse,
		config.Flags.UniqueLevel,
		config.Flags.Contrast,
		config.Position,
		internal.GetColor(config.Colors.SelectForeground),
		internal.GetColor(config.Colors.SelectBackground),
		internal.GetColor(config.Colors.MultiForeground),
		internal.GetColor(config.Colors.MultiBackground),
		internal.GetColor(config.Colors.Foreground),
		internal.GetColor(config.Colors.Background),
		internal.GetColor(config.Colors.HintForeground),
		internal.GetColor(config.Colors.HintBackground),
	)

	selected := viewbox.Present()
	if len(selected) == 0 {
		// slient here
		return nil
		// return fmt.Errorf("no selection made")

	}

	output, err := processResults(selected, config.Format)
	if err != nil {
		return err
	}

	return writeOutput(target, output)
}

func main() {
	debug.SetGCPercent(-1)

	arg := &Arguemnt{}
	var configPath string

	rootCmd := &cobra.Command{
		Use:   appName,
		Short: "Intelligent assistant for picking from terminal output",
		Long: color.New(color.FgHiMagenta).Sprintf(
			"Your intelligent assistant for picking from terminal output. %s",
			color.New(color.FgBlue).Sprintf("(%s)", FullVersion),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			var config *Config

			if arg.showVersion {
				fmt.Printf("%s version: %s\n", appName, FullVersion)
				return nil
			}

			// Skip config loading if configPath is "NONE"
			if configPath == "NONE" {
				config = NewDefaultConfig()
			} else {
				// Load configuration from TOML and defaults
				config, err = loadConfig(configPath)
				if err != nil {
					return fmt.Errorf("loading configuration: %w", err)
				}

			}

			// Apply CLI overrides
			applyCliOverrides(config, arg)

			return runApp(config, arg.inputFile, arg.target)
		},
	}

	// Configuration
	rootCmd.Flags().StringVar(&configPath, "config", "", "Config file path (default: XDG config dir, use 'NONE' to disable)")

	// Core settings
	rootCmd.Flags().StringVarP(&arg.alphabet, "alphabet", "a", "qwerty", "Sets the alphabet")
	rootCmd.Flags().StringVarP(&arg.format, "format", "f", "%H", "Specifies the out format for the picked hint")
	rootCmd.Flags().StringVarP(&arg.position, "position", "p", "left", "Hint position")
	rootCmd.Flags().StringArrayVarP(&arg.regexpPatterns, "regexp", "x", nil, "Use this regexp as extra pattern to match")

	// Colors
	rootCmd.Flags().StringVar(&arg.colors.Foreground, "fg-color", "green", "Sets the foreground color for matches")
	rootCmd.Flags().StringVar(&arg.colors.Background, "bg-color", "black", "Sets the background color for matches")
	rootCmd.Flags().StringVar(&arg.colors.HintForeground, "hint-fg-color", "yellow", "Sets the foreground color for hints")
	rootCmd.Flags().StringVar(&arg.colors.HintBackground, "hint-bg-color", "black", "Sets the background color for hints")
	rootCmd.Flags().StringVar(&arg.colors.MultiForeground, "multi-fg-color", "yellow", "Sets the foreground color for multi selected items")
	rootCmd.Flags().StringVar(&arg.colors.MultiBackground, "multi-bg-color", "black", "Sets the background color for multi selected items")
	rootCmd.Flags().StringVar(&arg.colors.SelectForeground, "select-fg-color", "blue", "Sets the foreground color for selection")
	rootCmd.Flags().StringVar(&arg.colors.SelectBackground, "select-bg-color", "black", "Sets the background color for selection")

	// Flags
	rootCmd.Flags().BoolVarP(&arg.flags.Multi, "multi", "m", false, "Enable multi-selection")
	rootCmd.Flags().BoolVarP(&arg.flags.Reverse, "reverse", "r", false, "Reverse the order for assigned hints")
	rootCmd.Flags().CountVarP(&arg.flags.UniqueLevel, "unique", "u", "Don't show duplicated hints for the same match (use -u for unique hints, -uu for unique match)")
	rootCmd.Flags().BoolVarP(&arg.flags.Contrast, "contrast", "c", false, "Put square brackets around hint for visibility")

	// Runtime settings
	rootCmd.Flags().StringVarP(&arg.target, "target", "t", "", "Stores the hint in the specified path")
	rootCmd.Flags().StringVarP(&arg.inputFile, "input-file", "i", "", "Read input from file instead of stdin")
	rootCmd.Flags().BoolVarP(&arg.showVersion, "version", "v", false, "Print version and exit")

	rootCmd.SetHelpTemplate(cmd.HelpTemplate)
	rootCmd.SetUsageFunc(func(c *cobra.Command) error {
		return cmd.ColorUsageFunc(c.OutOrStderr(), c)
	})

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Error executing command", "error", err)
		os.Exit(1)
	}
}
