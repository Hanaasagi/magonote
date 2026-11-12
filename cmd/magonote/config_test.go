package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigDisablesDetectionPlugins(t *testing.T) {
	config := NewDefaultConfig()
	if config.Plugins.Profiles != nil || config.Rules.Words {
		t.Fatalf("default detection plugins are enabled: %#v", config)
	}
}

func TestProfilesExcludePromptDefaultsAndExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[plugins.profiles]\nenabled=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfigFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Plugins.Profiles == nil || !profilesExcludePrompt(config.Plugins.Profiles) {
		t.Fatalf("default exclude_prompt = %#v", config.Plugins.Profiles)
	}

	if err := os.WriteFile(path, []byte("[plugins.profiles]\nenabled=true\nexclude_prompt=false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config, err = LoadConfigFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if profilesExcludePrompt(config.Plugins.Profiles) {
		t.Fatalf("explicit exclude_prompt=false was ignored")
	}
}
