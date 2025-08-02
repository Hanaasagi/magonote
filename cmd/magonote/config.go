package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Core    CoreConfig    `toml:"core"`
	Regexp  RegexpConfig  `toml:"regexp"`
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

type RegexpConfig struct {
	Patterns []string `toml:"patterns"`
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

type ExclusionRule struct {
	Type    string `toml:"type"`    // "regex" or "text"
	Pattern string `toml:"pattern"` // The pattern or text to exclude
}

type ExclusionConfig struct {
	Enabled bool            `toml:"enabled"`
	Rules   []ExclusionRule `toml:"rules"`
}

type PluginsConfig struct {
	Tabledetection *TableDetectionPluginConfig `toml:"tabledetection"`
	Colordetection *ColorDetectionPluginConfig `toml:"colordetection"`
	Exclusion      *ExclusionConfig            `toml:"exclusion"`
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
		Regexp: RegexpConfig{
			Patterns: []string{},
		},
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
			Exclusion:      nil,
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
