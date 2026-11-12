package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/profile"
	"github.com/spf13/cobra"
)

func TestWordsCLIOverridesOnlyWhenChanged(t *testing.T) {
	config := NewDefaultConfig()
	config.Rules.Words = true
	args := &Arguments{}
	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&args.words, "words", false, "")
	applyCliOverrides(cmd, config, args)
	if !config.Rules.Words {
		t.Fatalf("unset --words cleared TOML value")
	}
	if err := cmd.Flags().Set("words", "false"); err != nil {
		t.Fatal(err)
	}
	applyCliOverrides(cmd, config, args)
	if config.Rules.Words {
		t.Fatalf("explicit --words=false did not override TOML")
	}
}

func TestRunDetectOutputsDisplaySpanJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("中 https://example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := &Arguments{inputFile: path}
	var output bytes.Buffer
	if err := runDetect(NewDefaultConfig(), args, true, &output); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Entities []entity.Entity `json:"entities"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not JSON: %s: %v", output.String(), err)
	}
	if len(decoded.Entities) != 1 || decoded.Entities[0].Span.StartCol != 3 {
		t.Fatalf("entities = %#v", decoded.Entities)
	}
}

func TestRunDetectUsesWordsAndProfilesConfig(t *testing.T) {
	config := NewDefaultConfig()
	config.Rules.Words = true
	config.Plugins.Profiles = &ProfilesPluginConfig{Enabled: true}
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("$ ls\nfoo.txt"), 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runDetect(config, &Arguments{inputFile: path}, true, &output); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Prompts []profile.Prompt `json:"prompts"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil || len(decoded.Prompts) != 1 {
		t.Fatalf("profile output = %s, err=%v", output.String(), err)
	}
}

func TestRunDetectRequiresJSONFlag(t *testing.T) {
	if err := runDetect(NewDefaultConfig(), &Arguments{}, false, &bytes.Buffer{}); err == nil {
		t.Fatal("detect without --json succeeded")
	}
}

func TestDetectCommandRegistersSharedFlags(t *testing.T) {
	args := &Arguments{}
	configPath := ""
	cmd := newDetectCommand(&configPath, args)
	for _, name := range []string{"config", "input-file", "regexp", "words", "json"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("detect flag %q is missing", name)
		}
	}
}
