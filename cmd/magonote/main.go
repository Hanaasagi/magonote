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

	"github.com/BurntSushi/toml"
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

// TomlConfig represents the TOML configuration file structure
type TomlConfig struct {
	Alphabet string `toml:"alphabet"`
	Format   string `toml:"format"`
	Position string `toml:"position"`
	Regexp   struct {
		Patterns []string `toml:"patterns"`
	} `toml:"regexp"`
	Colors struct {
		Foreground       string `toml:"foreground"`
		Background       string `toml:"background"`
		HintForeground   string `toml:"hint_foreground"`
		HintBackground   string `toml:"hint_background"`
		MultiForeground  string `toml:"multi_foreground"`
		MultiBackground  string `toml:"multi_background"`
		SelectForeground string `toml:"select_foreground"`
		SelectBackground string `toml:"select_background"`
	} `toml:"colors"`
	Flags struct {
		Multi       bool `toml:"multi"`
		Reverse     bool `toml:"reverse"`
		UniqueLevel int  `toml:"unique_level"`
		Contrast    bool `toml:"contrast"`
	} `toml:"flags"`
}

// Config holds the merged application configuration
type Config struct {
	// Core settings
	alphabet       string
	format         string
	position       string
	regexpPatterns []string

	// Colors
	colors ColorConfig

	// Flags
	flags FlagConfig

	// Runtime-only settings (not in TOML)
	target      string
	inputFile   string
	showVersion bool
	configPath  string
}

// AppConfig holds application configuration
type AppConfig struct {
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

// ColorConfig groups color-related configuration
type ColorConfig struct {
	foreground       string
	background       string
	hintForeground   string
	hintBackground   string
	multiForeground  string
	multiBackground  string
	selectForeground string
	selectBackground string
}

// FlagConfig groups boolean flags
type FlagConfig struct {
	multi       bool
	reverse     bool
	uniqueLevel int // 0: none, 1: unique hints, 2: highlight only one duplicate
	contrast    bool
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

// getDefaultConfig returns the default configuration
func getDefaultConfig() *Config {
	return &Config{
		alphabet: "qwerty",
		format:   "%H",
		position: "left",
		colors: ColorConfig{
			foreground:       "green",
			background:       "black",
			hintForeground:   "yellow",
			hintBackground:   "black",
			multiForeground:  "yellow",
			multiBackground:  "black",
			selectForeground: "blue",
			selectBackground: "black",
		},
		flags: FlagConfig{
			multi:       false,
			reverse:     false,
			uniqueLevel: 0,
			contrast:    false,
		},
	}
}

// loadTomlConfig loads configuration from a TOML file
func loadTomlConfig(configPath string) (*TomlConfig, error) {
	var config TomlConfig

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil // Config file doesn't exist, return nil
	}

	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		return nil, fmt.Errorf("failed to decode TOML config: %w", err)
	}

	return &config, nil
}

// mergeConfigs merges TOML config with default config
func mergeConfigs(base *Config, tomlConfig *TomlConfig) {
	if tomlConfig == nil {
		return
	}

	if tomlConfig.Alphabet != "" {
		base.alphabet = tomlConfig.Alphabet
	}
	if tomlConfig.Format != "" {
		base.format = tomlConfig.Format
	}
	if tomlConfig.Position != "" {
		base.position = tomlConfig.Position
	}
	if len(tomlConfig.Regexp.Patterns) > 0 {
		base.regexpPatterns = tomlConfig.Regexp.Patterns
	}

	// Colors
	if tomlConfig.Colors.Foreground != "" {
		base.colors.foreground = tomlConfig.Colors.Foreground
	}
	if tomlConfig.Colors.Background != "" {
		base.colors.background = tomlConfig.Colors.Background
	}
	if tomlConfig.Colors.HintForeground != "" {
		base.colors.hintForeground = tomlConfig.Colors.HintForeground
	}
	if tomlConfig.Colors.HintBackground != "" {
		base.colors.hintBackground = tomlConfig.Colors.HintBackground
	}
	if tomlConfig.Colors.MultiForeground != "" {
		base.colors.multiForeground = tomlConfig.Colors.MultiForeground
	}
	if tomlConfig.Colors.MultiBackground != "" {
		base.colors.multiBackground = tomlConfig.Colors.MultiBackground
	}
	if tomlConfig.Colors.SelectForeground != "" {
		base.colors.selectForeground = tomlConfig.Colors.SelectForeground
	}
	if tomlConfig.Colors.SelectBackground != "" {
		base.colors.selectBackground = tomlConfig.Colors.SelectBackground
	}

	// Flags - only merge if explicitly set in TOML
	base.flags.multi = tomlConfig.Flags.Multi
	base.flags.reverse = tomlConfig.Flags.Reverse
	base.flags.uniqueLevel = tomlConfig.Flags.UniqueLevel
	base.flags.contrast = tomlConfig.Flags.Contrast
}

// loadConfig loads and merges configuration from multiple sources
func loadConfig(configPath string) (*Config, error) {
	config := getDefaultConfig()

	// Skip config loading if configPath is "NONE"
	if configPath == "NONE" {
		return config, nil
	}

	// Determine config file path
	var actualConfigPath string
	if configPath != "" {
		actualConfigPath = configPath
	} else {
		// Use XDG config directory
		actualConfigPath = filepath.Join(xdg.ConfigHome, appName, "config.toml")
	}

	config.configPath = actualConfigPath

	// Load TOML config
	tomlConfig, err := loadTomlConfig(actualConfigPath)
	if err != nil {
		return nil, fmt.Errorf("loading config from %s: %w", actualConfigPath, err)
	}

	// Merge TOML config with defaults
	mergeConfigs(config, tomlConfig)

	return config, nil
}

// applyCliOverrides applies CLI arguments to override config values
func applyCliOverrides(config *Config, cliConfig *AppConfig) {
	// Apply CLI overrides only if they were explicitly set
	// Note: This is a simplified approach - in a real implementation,
	// you might want to track which flags were actually set by the user

	if cliConfig.alphabet != "qwerty" { // Default value check
		config.alphabet = cliConfig.alphabet
	}
	if cliConfig.format != "%H" { // Default value check
		config.format = cliConfig.format
	}
	if cliConfig.position != "left" { // Default value check
		config.position = cliConfig.position
	}
	if len(cliConfig.regexpPatterns) > 0 {
		config.regexpPatterns = cliConfig.regexpPatterns
	}

	// Colors - apply if different from defaults
	if cliConfig.colors.foreground != "green" {
		config.colors.foreground = cliConfig.colors.foreground
	}
	if cliConfig.colors.background != "black" {
		config.colors.background = cliConfig.colors.background
	}
	if cliConfig.colors.hintForeground != "yellow" {
		config.colors.hintForeground = cliConfig.colors.hintForeground
	}
	if cliConfig.colors.hintBackground != "black" {
		config.colors.hintBackground = cliConfig.colors.hintBackground
	}
	if cliConfig.colors.multiForeground != "yellow" {
		config.colors.multiForeground = cliConfig.colors.multiForeground
	}
	if cliConfig.colors.multiBackground != "black" {
		config.colors.multiBackground = cliConfig.colors.multiBackground
	}
	if cliConfig.colors.selectForeground != "blue" {
		config.colors.selectForeground = cliConfig.colors.selectForeground
	}
	if cliConfig.colors.selectBackground != "black" {
		config.colors.selectBackground = cliConfig.colors.selectBackground
	}

	// Flags - always apply from CLI since they might override config
	config.flags.multi = cliConfig.flags.multi
	config.flags.reverse = cliConfig.flags.reverse
	config.flags.uniqueLevel = cliConfig.flags.uniqueLevel
	config.flags.contrast = cliConfig.flags.contrast

	// Runtime settings
	config.target = cliConfig.target
	config.inputFile = cliConfig.inputFile
	config.showVersion = cliConfig.showVersion
}

// runApp runs the main application logic
func runApp(config *Config) error {
	if config.showVersion {
		fmt.Printf("%s version: %s\n", appName, FullVersion)
		return nil
	}

	lines, err := readInput(config.inputFile)
	if err != nil {
		return err
	}

	state := internal.NewState(lines, config.alphabet, config.regexpPatterns)
	viewbox := internal.NewView(
		state,
		config.flags.multi,
		config.flags.reverse,
		config.flags.uniqueLevel,
		config.flags.contrast,
		config.position,
		internal.GetColor(config.colors.selectForeground),
		internal.GetColor(config.colors.selectBackground),
		internal.GetColor(config.colors.multiForeground),
		internal.GetColor(config.colors.multiBackground),
		internal.GetColor(config.colors.foreground),
		internal.GetColor(config.colors.background),
		internal.GetColor(config.colors.hintForeground),
		internal.GetColor(config.colors.hintBackground),
	)

	selected := viewbox.Present()
	if len(selected) == 0 {
		// slient here
		return nil
		// return fmt.Errorf("no selection made")

	}

	output, err := processResults(selected, config.format)
	if err != nil {
		return err
	}

	return writeOutput(config.target, output)
}

func main() {
	debug.SetGCPercent(-1)

	cliConfig := &AppConfig{}
	var configPath string

	rootCmd := &cobra.Command{
		Use:   appName,
		Short: "Intelligent assistant for picking from terminal output",
		Long: color.New(color.FgHiMagenta).Sprintf(
			"Your intelligent assistant for picking from terminal output. %s",
			color.New(color.FgBlue).Sprintf("(%s)", FullVersion),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration from TOML and defaults
			config, err := loadConfig(configPath)
			if err != nil {
				return fmt.Errorf("loading configuration: %w", err)
			}

			// Apply CLI overrides
			applyCliOverrides(config, cliConfig)

			return runApp(config)
		},
	}

	// Configuration
	rootCmd.Flags().StringVar(&configPath, "config", "", "Config file path (default: XDG config dir, use 'NONE' to disable)")

	// Core settings
	rootCmd.Flags().StringVarP(&cliConfig.alphabet, "alphabet", "a", "qwerty", "Sets the alphabet")
	rootCmd.Flags().StringVarP(&cliConfig.format, "format", "f", "%H", "Specifies the out format for the picked hint")
	rootCmd.Flags().StringVarP(&cliConfig.position, "position", "p", "left", "Hint position")
	rootCmd.Flags().StringArrayVarP(&cliConfig.regexpPatterns, "regexp", "x", nil, "Use this regexp as extra pattern to match")

	// Colors
	rootCmd.Flags().StringVar(&cliConfig.colors.foreground, "fg-color", "green", "Sets the foreground color for matches")
	rootCmd.Flags().StringVar(&cliConfig.colors.background, "bg-color", "black", "Sets the background color for matches")
	rootCmd.Flags().StringVar(&cliConfig.colors.hintForeground, "hint-fg-color", "yellow", "Sets the foreground color for hints")
	rootCmd.Flags().StringVar(&cliConfig.colors.hintBackground, "hint-bg-color", "black", "Sets the background color for hints")
	rootCmd.Flags().StringVar(&cliConfig.colors.multiForeground, "multi-fg-color", "yellow", "Sets the foreground color for multi selected items")
	rootCmd.Flags().StringVar(&cliConfig.colors.multiBackground, "multi-bg-color", "black", "Sets the background color for multi selected items")
	rootCmd.Flags().StringVar(&cliConfig.colors.selectForeground, "select-fg-color", "blue", "Sets the foreground color for selection")
	rootCmd.Flags().StringVar(&cliConfig.colors.selectBackground, "select-bg-color", "black", "Sets the background color for selection")

	// Flags
	rootCmd.Flags().BoolVarP(&cliConfig.flags.multi, "multi", "m", false, "Enable multi-selection")
	rootCmd.Flags().BoolVarP(&cliConfig.flags.reverse, "reverse", "r", false, "Reverse the order for assigned hints")
	rootCmd.Flags().CountVarP(&cliConfig.flags.uniqueLevel, "unique", "u", "Don't show duplicated hints for the same match (use -u for unique hints, -uu for unique match)")
	rootCmd.Flags().BoolVarP(&cliConfig.flags.contrast, "contrast", "c", false, "Put square brackets around hint for visibility")

	// Runtime settings
	rootCmd.Flags().StringVarP(&cliConfig.target, "target", "t", "", "Stores the hint in the specified path")
	rootCmd.Flags().StringVarP(&cliConfig.inputFile, "input-file", "i", "", "Read input from file instead of stdin")
	rootCmd.Flags().BoolVarP(&cliConfig.showVersion, "version", "v", false, "Print version and exit")

	rootCmd.SetHelpTemplate(cmd.HelpTemplate)
	rootCmd.SetUsageFunc(func(c *cobra.Command) error {
		return cmd.ColorUsageFunc(c.OutOrStderr(), c)
	})

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Error executing command", "error", err)
		os.Exit(1)
	}
}
