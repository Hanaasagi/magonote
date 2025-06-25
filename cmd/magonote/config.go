package main

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
)

type Config struct {
	Core   CoreConfig   `toml:"core"`
	Regexp RegexpConfig `toml:"regexp"`
	Colors ColorConfig  `toml:"colors"`
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
