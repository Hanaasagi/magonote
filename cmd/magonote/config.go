package main

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
)

type ColorConfig struct {
	Foreground       string `toml:"foreground"`
	Background       string `toml:"background"`
	HintForeground   string `toml:"hint_foreground"`
	HintBackground   string `toml:"hint_background"`
	MultiForeground  string `toml:"multi_foreground"`
	MultiBackground  string `toml:"multi_background"`
	SelectForeground string `toml:"select_foreground"`
	SelectBackground string `toml:"select_background"`
}

type FlagConfig struct {
	Multi       bool `toml:"multi"`
	Reverse     bool `toml:"reverse"`
	UniqueLevel int  `toml:"unique_level"`
	Contrast    bool `toml:"contrast"`
}

type Config struct {
	Alphabet string `toml:"alphabet"`
	Format   string `toml:"format"`
	Position string `toml:"position"`

	RegexpPatterns []string    `toml:"regexp_patterns"`
	Colors         ColorConfig `toml:"colors"`
	Flags          FlagConfig  `toml:"flags"`
}

func NewDefaultConfig() *Config {
	return &Config{
		Alphabet:       "qwerty",
		Format:         "%H",
		Position:       "left",
		RegexpPatterns: []string{},
		Colors: ColorConfig{
			Foreground:       "green",
			Background:       "black",
			HintForeground:   "yellow",
			HintBackground:   "black",
			MultiForeground:  "yellow",
			MultiBackground:  "black",
			SelectForeground: "blue",
			SelectBackground: "black",
		},
		Flags: FlagConfig{
			Multi:       false,
			Reverse:     false,
			UniqueLevel: 0,
			Contrast:    false,
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
