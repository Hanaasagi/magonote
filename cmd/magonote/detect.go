package main

import (
	"fmt"
	"io"

	"github.com/Hanaasagi/magonote/pkg/textdetection"
	"github.com/Hanaasagi/magonote/pkg/textdetection/profile"
	"github.com/Hanaasagi/magonote/pkg/textdetection/table"
	"github.com/spf13/cobra"
)

func newDetectCommand(configPath *string, args *Arguments) *cobra.Command {
	jsonOutput := false
	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Detect terminal entities and write JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var config *Config
			var err error
			if *configPath == "NONE" {
				config = NewDefaultConfig()
			} else {
				config, err = loadConfig(*configPath)
				if err != nil {
					return fmt.Errorf("loading configuration: %w", err)
				}
			}
			applyCliOverrides(cmd, config, args)
			return runDetect(config, args, jsonOutput, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(configPath, "config", "", "Config file path (default: XDG config dir, use 'NONE' to disable)")
	cmd.Flags().StringVarP(&args.inputFile, "input-file", "i", "", "Read input from file instead of stdin")
	cmd.Flags().StringArrayVarP(&args.regexpPatterns, "regexp", "x", nil, "Use this regexp as extra pattern to match")
	cmd.Flags().BoolVar(&args.words, "words", false, "Enable fallback word candidates")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write detection results as JSON")
	return cmd
}

func runDetect(config *Config, args *Arguments, jsonOutput bool, output io.Writer) error {
	if !jsonOutput {
		return fmt.Errorf("detect requires --json")
	}
	input, err := readInput(args.inputFile)
	if err != nil {
		return err
	}
	result, err := textdetection.Detect(input, detectionOptions(config))
	if err != nil {
		return err
	}
	data, err := result.JSON(false)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, string(data))
	return err
}

func detectionOptions(config *Config) textdetection.Options {
	opts := textdetection.Options{Words: config.Rules.Words}
	for _, rule := range config.Rules.Include.Rules {
		if rule.Type == "regex" && rule.Pattern != "" {
			opts.CustomPatterns = append(opts.CustomPatterns, rule.Pattern)
		}
	}
	for _, rule := range config.Rules.Exclude.Rules {
		opts.Exclusions = append(opts.Exclusions, textdetection.Exclusion{Type: rule.Type, Pattern: rule.Pattern})
	}
	plugins := config.Plugins
	if plugins.Tabledetection != nil && plugins.Tabledetection.Enabled {
		opts.Table = &table.Options{
			MinRows: plugins.Tabledetection.MinLines, MinColumns: plugins.Tabledetection.MinColumns,
			ConfidenceThreshold: plugins.Tabledetection.ConfidenceThreshold,
		}
	}
	opts.Style = plugins.Colordetection != nil && plugins.Colordetection.Enabled
	if plugins.Profiles != nil && plugins.Profiles.Enabled {
		opts.Profiles = &profile.Options{ExcludePrompt: profilesExcludePrompt(plugins.Profiles), PromptPatterns: plugins.Profiles.PromptPatterns}
	}
	return opts
}
