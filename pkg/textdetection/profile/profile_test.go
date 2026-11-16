package profile

import "testing"

func TestDockerCommandArgsTruncatedOption(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want int // len of result; 0 means nil
	}{
		{"normal", []string{"docker", "--context", "ctx", "ps"}, 1},
		{"truncated option", []string{"docker", "--context"}, 0},
		{"flag only", []string{"docker", "--debug", "ps"}, 1},
		{"no args", []string{"docker"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dockerCommandArgs(tt.argv)
			if len(got) != tt.want {
				t.Errorf("dockerCommandArgs(%v) = %v (len %d), want len %d", tt.argv, got, len(got), tt.want)
			}
		})
	}
}

func TestKubectlResourceTruncatedOption(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{"normal", []string{"kubectl", "get", "pods"}, "pod"},
		{"with option", []string{"kubectl", "get", "-n", "ns", "pods"}, "pod"},
		{"truncated option", []string{"kubectl", "get", "-n"}, "resource"},
		{"option only", []string{"kubectl", "get", "-o", "json", "deploy"}, "deployment"},
		{"no resource", []string{"kubectl", "get"}, "resource"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kubectlResource(tt.argv); got != tt.want {
				t.Errorf("kubectlResource(%v) = %q, want %q", tt.argv, got, tt.want)
			}
		})
	}
}

func TestHasBundledFlag(t *testing.T) {
	tests := []struct {
		arg  string
		flag byte
		want bool
	}{
		{"-s", 's', true},
		{"-sb", 's', true},
		{"-bs", 's', true},
		{"-b", 's', false},
		{"--short", 's', false},
		{"-", 's', false},
		{"s", 's', false},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			if got := hasBundledFlag(tt.arg, tt.flag); got != tt.want {
				t.Errorf("hasBundledFlag(%q, %q) = %v, want %v", tt.arg, tt.flag, got, tt.want)
			}
		})
	}
}
