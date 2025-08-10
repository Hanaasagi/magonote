package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Core    CoreConfig    `toml:"core"`
	Rules   RulesConfig   `toml:"rules"`
	Colors  ColorConfig   `toml:"colors"`
	Plugins PluginsConfig `toml:"plugins"`
}

type CoreConfig struct {
	Alphabet    string `toml:"alphabet"`
	Format      string `toml:"format"`
	Position    string `toml:"position"`
	Multi       bool   `toml:"multi"`
	Reverse     bool   `toml:"reverse"`
	UniqueLevel int    `toml:"unique_level"`
	Contrast    bool   `toml:"contrast"`
}

// RulesConfig unifies user-defined include (match) and exclude (filter) rules
// Both include and exclude carry a "rules" list with items of the same shape: { type, pattern }
type RulesConfig struct {
	Include RulesList `toml:"include"`
	Exclude RulesList `toml:"exclude"`
}

// RulesList groups a list of rules under a section
type RulesList struct {
	Rules []Rule `toml:"rules"`
}

type ColorGroup struct {
	Foreground string `toml:"foreground"`
	Background string `toml:"background"`
}

type ColorConfig struct {
	Match  ColorGroup `toml:"match"`
	Hint   ColorGroup `toml:"hint"`
	Multi  ColorGroup `toml:"multi"`
	Select ColorGroup `toml:"select"`
}

type TableDetectionPluginConfig struct {
	Enabled             bool    `toml:"enabled"`
	MinLines            int     `toml:"min_lines"`
	MinColumns          int     `toml:"min_columns"`
	ConfidenceThreshold float64 `toml:"confidence_threshold"`
}

type ColorDetectionPluginConfig struct {
	Enabled bool `toml:"enabled"`
}

// Rule describes a single rule item used in include/exclude lists
type Rule struct {
	Type    string `toml:"type"`    // "regex" or "text"
	Pattern string `toml:"pattern"` // The pattern or text to exclude
}

type PluginsConfig struct {
	Tabledetection *TableDetectionPluginConfig `toml:"tabledetection"`
	Colordetection *ColorDetectionPluginConfig `toml:"colordetection"`
}

func NewDefaultConfig() *Config {
	return &Config{
		Core: CoreConfig{
			Alphabet:    "qwerty",
			Format:      "%H",
			Position:    "left",
			Multi:       false,
			Reverse:     false,
			UniqueLevel: 0,
			Contrast:    false,
		},
		Rules: RulesConfig{Include: RulesList{Rules: []Rule{}}, Exclude: RulesList{Rules: []Rule{}}},
		Colors: ColorConfig{
			Match: ColorGroup{
				Foreground: "green",
				Background: "black",
			},
			Hint: ColorGroup{
				Foreground: "yellow",
				Background: "black",
			},
			Multi: ColorGroup{
				Foreground: "yellow",
				Background: "black",
			},
			Select: ColorGroup{
				Foreground: "blue",
				Background: "black",
			},
		},
		Plugins: PluginsConfig{
			Tabledetection: nil,
			Colordetection: nil,
		},
	}
}

func LoadConfigFromFile(path string) (*Config, error) {
	config := NewDefaultConfig()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return config, nil // no config file, return defaults
	}

	if _, err := toml.DecodeFile(path, config); err != nil {
		return nil, fmt.Errorf("failed to decode TOML config: %w", err)
	}
	return config, nil
}
