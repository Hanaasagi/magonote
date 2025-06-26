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

// runApp runs the main application logic
func runApp(config *AppConfig) error {
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

	config := &AppConfig{}

	rootCmd := &cobra.Command{
		Use:   appName,
		Short: "Intelligent assistant for picking from terminal output",
		Long: color.New(color.FgHiMagenta).Sprintf(
			"Your intelligent assistant for picking from terminal output. %s",
			color.New(color.FgBlue).Sprintf("(%s)", FullVersion),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApp(config)
		},
	}

	rootCmd.Flags().StringVarP(&config.alphabet, "alphabet", "a", "qwerty", "Sets the alphabet")
	rootCmd.Flags().StringVarP(&config.format, "format", "f", "%H", "Specifies the out format for the picked hint")
	rootCmd.Flags().StringVar(&config.colors.foreground, "fg-color", "green", "Sets the foreground color for matches")
	rootCmd.Flags().StringVar(&config.colors.background, "bg-color", "black", "Sets the background color for matches")
	rootCmd.Flags().StringVar(&config.colors.hintForeground, "hint-fg-color", "yellow", "Sets the foreground color for hints")
	rootCmd.Flags().StringVar(&config.colors.hintBackground, "hint-bg-color", "black", "Sets the background color for hints")
	rootCmd.Flags().StringVar(&config.colors.multiForeground, "multi-fg-color", "yellow", "Sets the foreground color for multi selected items")
	rootCmd.Flags().StringVar(&config.colors.multiBackground, "multi-bg-color", "black", "Sets the background color for multi selected items")
	rootCmd.Flags().StringVar(&config.colors.selectForeground, "select-fg-color", "blue", "Sets the foreground color for selection")
	rootCmd.Flags().StringVar(&config.colors.selectBackground, "select-bg-color", "black", "Sets the background color for selection")
	rootCmd.Flags().BoolVarP(&config.flags.multi, "multi", "m", false, "Enable multi-selection")
	rootCmd.Flags().BoolVarP(&config.flags.reverse, "reverse", "r", false, "Reverse the order for assigned hints")
	rootCmd.Flags().CountVarP(&config.flags.uniqueLevel, "unique", "u", "Don't show duplicated hints for the same match (use -u for unique hints, -uu for unique match)")
	rootCmd.Flags().StringVarP(&config.position, "position", "p", "left", "Hint position")
	rootCmd.Flags().StringArrayVarP(&config.regexpPatterns, "regexp", "x", nil, "Use this regexp as extra pattern to match")
	rootCmd.Flags().BoolVarP(&config.flags.contrast, "contrast", "c", false, "Put square brackets around hint for visibility")
	rootCmd.Flags().StringVarP(&config.target, "target", "t", "", "Stores the hint in the specified path")
	rootCmd.Flags().StringVarP(&config.inputFile, "input-file", "i", "", "Read input from file instead of stdin")
	rootCmd.Flags().BoolVarP(&config.showVersion, "version", "v", false, "Print version and exit")

	rootCmd.SetHelpTemplate(cmd.HelpTemplate)
	rootCmd.SetUsageFunc(func(c *cobra.Command) error {
		return cmd.ColorUsageFunc(c.OutOrStderr(), c)
	})

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Error executing command", "error", err)
		os.Exit(1)
	}
}
