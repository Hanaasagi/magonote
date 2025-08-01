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

type Arguments struct {
	alphabet       string
	format         string
	position       string
	regexpPatterns []string
	multi          bool
	reverse        bool
	uniqueLevel    int // 0: none, 1: unique hints, 2: highlight only one duplicate
	contrast       bool
	target         string
	inputFile      string
	showVersion    bool
	listView       bool

	// colors
	foregroundColor       string
	backgroundColor       string
	hintForegroundColor   string
	hintBackgroundColor   string
	multiForegroundColor  string
	multiBackgroundColor  string
	selectForegroundColor string
	selectBackgroundColor string
}

func init() {
	// Initialize logging
	if err := os.MkdirAll(appDir, 0755); err != nil {
		panic(fmt.Sprintf("Error creating log directory: %v", err))
	}

	logFilePath := filepath.Join(appDir, appName+".log")

	logLevel := os.Getenv("MAGONOTE_LOG")
	if logLevel == "" {
		if internal.IsDebugMode() {
			logLevel = "debug"
		} else {
			logLevel = "info"
		}
	}

	logger.InitLogger(logFilePath, logLevel)

	// Initialize crash reporting
	crashFilePath := filepath.Join(appDir, "crash")
	if f, err := os.Create(crashFilePath); err == nil {
		_ = debug.SetCrashOutput(f, debug.CrashOptions{})
	}
}

// readInput reads input from file or stdin with buffering
func readInput(inputFile string) (string, error) {
	var reader io.Reader
	var closer io.Closer

	if inputFile != "" {
		file, err := os.Open(inputFile)
		if err != nil {
			return "", fmt.Errorf("opening input file: %w", err)
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
	var content strings.Builder

	for {
		line, err := bufferedReader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("reading input: %w", err)
		}

		if line != "" {
			content.WriteString(line)
		}

		if err == io.EOF {
			break
		}
	}

	return strings.TrimSuffix(content.String(), "\n"), nil
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
func applyCliOverrides(cmd *cobra.Command, config *Config, args *Arguments) {
	// Core settings
	if cmd.Flags().Changed("alphabet") {
		config.Core.Alphabet = args.alphabet
	}
	if cmd.Flags().Changed("format") {
		config.Core.Format = args.format
	}
	if cmd.Flags().Changed("position") {
		config.Core.Position = args.position
	}

	if len(args.regexpPatterns) > 0 {
		config.Regexp.Patterns = args.regexpPatterns
	}

	if cmd.Flags().Changed("fg-color") {
		config.Colors.Match.Foreground = args.foregroundColor
	}
	if cmd.Flags().Changed("bg-color") {
		config.Colors.Match.Background = args.backgroundColor
	}
	if cmd.Flags().Changed("hint-fg-color") {
		config.Colors.Hint.Foreground = args.hintForegroundColor
	}
	if cmd.Flags().Changed("hint-bg-color") {
		config.Colors.Hint.Background = args.hintBackgroundColor
	}
	if cmd.Flags().Changed("multi-fg-color") {
		config.Colors.Multi.Foreground = args.multiForegroundColor
	}
	if cmd.Flags().Changed("multi-bg-color") {
		config.Colors.Multi.Background = args.multiBackgroundColor
	}
	if cmd.Flags().Changed("select-fg-color") {
		config.Colors.Select.Foreground = args.selectForegroundColor
	}
	if cmd.Flags().Changed("select-bg-color") {
		config.Colors.Select.Background = args.selectBackgroundColor
	}

	if cmd.Flags().Changed("multi") {
		config.Core.Multi = args.multi
	}
	if cmd.Flags().Changed("reverse") {
		config.Core.Reverse = args.reverse
	}
	if cmd.Flags().Changed("unique") {
		config.Core.UniqueLevel = args.uniqueLevel
	}
	if cmd.Flags().Changed("contrast") {
		config.Core.Contrast = args.contrast
	}
}

// runApp runs the main application logic
func runApp(config *Config, args *Arguments) error {

	text, err := readInput(args.inputFile)
	if err != nil {
		return err
	}

	state := internal.NewState(text, config.Core.Alphabet, config.Regexp.Patterns)

	plugins := config.Plugins
	if plugins.Tabledetection != nil && plugins.Tabledetection.Enabled {
		state.TableDetectionConfig = internal.NewTableDetectionConfig(
			plugins.Tabledetection.MinLines,
			plugins.Tabledetection.MinColumns,
			plugins.Tabledetection.ConfidenceThreshold,
		)

	}
	if plugins.Colordetection != nil && plugins.Colordetection.Enabled {
		state.ColorDetectionConfig = internal.NewColorDetectionConfig()
	}

	var selected []internal.ChosenMatch

	if args.listView {
		listView := internal.NewListView(
			state,
			config.Core.Multi,
			internal.GetColor(config.Colors.Select.Foreground),
			internal.GetColor(config.Colors.Select.Background),
			internal.GetColor(config.Colors.Multi.Foreground),
			internal.GetColor(config.Colors.Multi.Background),
			internal.GetColor(config.Colors.Match.Foreground),
			internal.GetColor(config.Colors.Match.Background),
			internal.GetColor(config.Colors.Hint.Foreground),
			internal.GetColor(config.Colors.Hint.Background),
		)
		selected = listView.Present()
	} else {
		// Use full screen view
		viewbox := internal.NewView(
			state,
			config.Core.Multi,
			config.Core.Reverse,
			config.Core.UniqueLevel,
			config.Core.Contrast,
			config.Core.Position,
			internal.GetColor(config.Colors.Select.Foreground),
			internal.GetColor(config.Colors.Select.Background),
			internal.GetColor(config.Colors.Multi.Foreground),
			internal.GetColor(config.Colors.Multi.Background),
			internal.GetColor(config.Colors.Match.Foreground),
			internal.GetColor(config.Colors.Match.Background),
			internal.GetColor(config.Colors.Hint.Foreground),
			internal.GetColor(config.Colors.Hint.Background),
		)
		selected = viewbox.Present()
	}

	if len(selected) == 0 {
		// slient here
		return nil
		// return fmt.Errorf("no selection made")

	}

	output, err := processResults(selected, config.Core.Format)
	if err != nil {
		return err
	}

	return writeOutput(args.target, output)
}

func main() {
	debug.SetGCPercent(-1)

	var configPath string
	args := &Arguments{}

	rootCmd := &cobra.Command{
		Use:   appName,
		Short: "Intelligent assistant for picking from terminal output",
		Long: color.New(color.FgHiMagenta).Sprintf(
			"Your intelligent assistant for picking from terminal output. %s",
			color.New(color.FgBlue).Sprintf("(%s)", FullVersion),
		),
		RunE: func(cmd *cobra.Command, _args []string) error {
			var err error
			var config *Config

			if args.showVersion {
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
			applyCliOverrides(cmd, config, args)

			return runApp(config, args)
		},
	}

	// Configuration
	rootCmd.Flags().StringVar(&configPath, "config", "", "Config file path (default: XDG config dir, use 'NONE' to disable)")

	// Core settings
	rootCmd.Flags().StringVarP(&args.alphabet, "alphabet", "a", "qwerty", "Sets the alphabet")
	rootCmd.Flags().StringVarP(&args.format, "format", "f", "%H", "Specifies the out format for the picked hint")
	rootCmd.Flags().StringVarP(&args.position, "position", "p", "left", "Hint position")
	rootCmd.Flags().StringArrayVarP(&args.regexpPatterns, "regexp", "x", nil, "Use this regexp as extra pattern to match")

	// Colors
	rootCmd.Flags().StringVar(&args.foregroundColor, "fg-color", "green", "Sets the foreground color for matches")
	rootCmd.Flags().StringVar(&args.backgroundColor, "bg-color", "black", "Sets the background color for matches")
	rootCmd.Flags().StringVar(&args.hintForegroundColor, "hint-fg-color", "yellow", "Sets the foreground color for hints")
	rootCmd.Flags().StringVar(&args.hintBackgroundColor, "hint-bg-color", "black", "Sets the background color for hints")
	rootCmd.Flags().StringVar(&args.multiForegroundColor, "multi-fg-color", "yellow", "Sets the foreground color for multi selected items")
	rootCmd.Flags().StringVar(&args.multiBackgroundColor, "multi-bg-color", "black", "Sets the background color for multi selected items")
	rootCmd.Flags().StringVar(&args.selectForegroundColor, "select-fg-color", "blue", "Sets the foreground color for selection")
	rootCmd.Flags().StringVar(&args.selectBackgroundColor, "select-bg-color", "black", "Sets the background color for selection")

	// Flags
	rootCmd.Flags().BoolVarP(&args.multi, "multi", "m", false, "Enable multi-selection")
	rootCmd.Flags().BoolVarP(&args.reverse, "reverse", "r", false, "Reverse the order for assigned hints")
	rootCmd.Flags().CountVarP(&args.uniqueLevel, "unique", "u", "Don't show duplicated hints for the same match (use -u for unique hints, -uu for unique match)")
	rootCmd.Flags().BoolVarP(&args.contrast, "contrast", "c", false, "Put square brackets around hint for visibility")

	// Runtime settings
	rootCmd.Flags().StringVarP(&args.target, "target", "t", "", "Stores the hint in the specified path")
	rootCmd.Flags().StringVarP(&args.inputFile, "input-file", "i", "", "Read input from file instead of stdin")
	rootCmd.Flags().BoolVarP(&args.showVersion, "version", "v", false, "Print version and exit")

	rootCmd.Flags().BoolVar(&args.listView, "list", false, "Enable list view")

	rootCmd.SetHelpTemplate(cmd.HelpTemplate)
	rootCmd.SetUsageFunc(func(c *cobra.Command) error {
		return cmd.ColorUsageFunc(c.OutOrStderr(), c)
	})

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Error executing command", "error", err)
		os.Exit(1)
	}
}
