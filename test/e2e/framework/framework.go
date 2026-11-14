package framework

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/creack/pty"
)

// ErrTimeout indicates the PTY process did not exit within the allowed time
// and was killed. Tests can use errors.Is(err, ErrTimeout) to distinguish
// a hung process from a normal (possibly non-zero) exit.
var ErrTimeout = errors.New("process timed out")

// findProjectRoot searches for the project root directory containing go.mod
func findProjectRoot(startDir string) string {
	dir := startDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			content, err := os.ReadFile(goModPath)
			if err == nil && strings.HasPrefix(strings.TrimSpace(string(content)), "module github.com/Hanaasagi/magonote\n") {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// Framework provides utilities for running e2e tests
type Framework struct {
	BinaryPath  string
	ProjectRoot string
	Timeout     time.Duration
}

// TestCase represents a single e2e test case (PTY-based interaction)
type TestCase struct {
	Name           string
	Input          string
	Args           []string
	Keys           string
	ExpectedOutput string
	Timeout        time.Duration
}

// TestResult represents the result of a test case
type TestResult struct {
	Name    string
	Passed  bool
	Error   string
	Output  string
	Elapsed time.Duration
}

// DetectEntity mirrors the JSON structure output by `magonote detect --json`
type DetectEntity struct {
	Kind       string            `json:"kind"`
	Text       string            `json:"text"`
	Span       DetectSpan        `json:"span"`
	Confidence float64           `json:"confidence"`
	Class      int               `json:"class"`
	Source     string            `json:"source"`
	Meta       map[string]string `json:"meta,omitempty"`
	Location   *FileLocation     `json:"location,omitempty"`
}

type DetectSpan struct {
	Line     int `json:"line"`
	StartCol int `json:"start_col"`
	EndCol   int `json:"end_col"`
}

type FileLocation struct {
	Path   string `json:"path"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// DetectTable mirrors pkg/textdetection/table.Table JSON shape.
type DetectTable struct {
	StartLine  int             `json:"start_line"`
	EndLine    int             `json:"end_line"`
	HeaderLine int             `json:"header_line"`
	Columns    json.RawMessage `json:"columns"`
	Rows       json.RawMessage `json:"rows"`
	Confidence float64         `json:"confidence"`
}

// DetectPrompt mirrors pkg/textdetection/profile.Prompt JSON shape.
type DetectPrompt struct {
	Line      int            `json:"line"`
	PrefixEnd int            `json:"prefix_end"`
	Command   DetectCommand  `json:"command"`
}

type DetectCommand struct {
	Raw  string   `json:"raw"`
	Argv []string `json:"argv"`
}

// DetectResult represents the parsed JSON output from detect --json
type DetectResult struct {
	Entities []DetectEntity `json:"entities"`
	Tables   []DetectTable  `json:"tables"`
	Prompts  []DetectPrompt `json:"prompts"`
}

// NewFramework creates a new e2e test framework
func NewFramework() *Framework {
	return &Framework{
		BinaryPath: "",
		Timeout:    5 * time.Second,
	}
}

// SetBinaryPath sets the path to the magonote binary
func (f *Framework) SetBinaryPath(path string) {
	f.BinaryPath = path
}

// BuildBinary builds the magonote binary for testing
func (f *Framework) BuildBinary() error {
	if f.BinaryPath != "" {
		return nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	projectRoot := findProjectRoot(wd)
	if projectRoot == "" {
		return fmt.Errorf("could not find project root directory from %s", wd)
	}
	f.ProjectRoot = projectRoot

	buildDir := filepath.Join(projectRoot, "build")
	binaryPath := filepath.Join(buildDir, "magonote")

	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/magonote")
	cmd.Dir = projectRoot

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to build binary: %w, output: %s", err, string(output))
	}

	f.BinaryPath = binaryPath
	return nil
}

// FixturePath returns the absolute path to a fixture file
func (f *Framework) FixturePath(name string) string {
	if f.ProjectRoot == "" {
		wd, _ := os.Getwd()
		f.ProjectRoot = findProjectRoot(wd)
	}
	return filepath.Join(f.ProjectRoot, "test", "e2e", "fixtures", name)
}

// RunDetect runs `magonote detect --json` and returns the parsed result.
// This does NOT require a PTY — it's a pure CLI invocation.
func (f *Framework) RunDetect(input string, extraArgs ...string) (*DetectResult, error) {
	if err := f.BuildBinary(); err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "magonote-detect-*.txt")
	if err != nil {
		return nil, fmt.Errorf("temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(input); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write temp: %w", err)
	}
	tmpFile.Close()

	args := []string{"detect", "--json", "-i", tmpFile.Name(), "--config", "NONE"}
	args = append(args, extraArgs...)

	cmd := exec.Command(f.BinaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("detect failed: %w, stderr: %s", err, stderr.String())
	}

	var result DetectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w, raw: %s", err, stdout.String())
	}
	return &result, nil
}

// RunDetectFile runs detect on an existing fixture file
func (f *Framework) RunDetectFile(fixturePath string, extraArgs ...string) (*DetectResult, error) {
	if err := f.BuildBinary(); err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}

	args := []string{"detect", "--json", "-i", fixturePath, "--config", "NONE"}
	args = append(args, extraArgs...)

	cmd := exec.Command(f.BinaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("detect failed: %w, stderr: %s", err, stderr.String())
	}

	var result DetectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w, raw: %s", err, stdout.String())
	}
	return &result, nil
}

// RunWithTarget runs the binary in PTY mode and captures output to a target file
// instead of stdout. Returns the content of the target file.
func (f *Framework) RunWithTarget(input string, keys string, extraArgs ...string) (string, error) {
	if err := f.BuildBinary(); err != nil {
		return "", fmt.Errorf("build: %w", err)
	}

	inputFile, err := os.CreateTemp("", "magonote-input-*.txt")
	if err != nil {
		return "", fmt.Errorf("input temp: %w", err)
	}
	defer os.Remove(inputFile.Name())

	if _, err := inputFile.WriteString(input); err != nil {
		inputFile.Close()
		return "", fmt.Errorf("write input: %w", err)
	}
	inputFile.Close()

	targetFile, err := os.CreateTemp("", "magonote-target-*.txt")
	if err != nil {
		return "", fmt.Errorf("target temp: %w", err)
	}
	targetPath := targetFile.Name()
	targetFile.Close()
	defer os.Remove(targetPath)

	args := []string{"-i", inputFile.Name(), "--config", "NONE", "-t", targetPath}
	args = append(args, extraArgs...)

	cmd := exec.Command(f.BinaryPath, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 50, Cols: 200})
	if err != nil {
		return "", fmt.Errorf("pty start: %w", err)
	}
	defer ptmx.Close()

	// Drain PTY output in background so cmd.Wait() doesn't hang
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := ptmx.Read(buf); err != nil {
				return
			}
		}
	}()

	time.Sleep(300 * time.Millisecond)

	if keys != "" {
		// Send keys as-is without appending newline. Callers supply
		// the exact bytes (e.g. "a" for hint selection, "\x1b" for
		// ESC). Appending \n after \x1b confuses tcell's escape-
		// sequence detection and causes spurious timeouts.
		if _, err := ptmx.Write([]byte(keys)); err != nil {
			return "", fmt.Errorf("send keys: %w", err)
		}
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var timedOut bool
	select {
	case <-done:
	case <-time.After(f.Timeout):
		cmd.Process.Kill()
		timedOut = true
	}

	content, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		if timedOut {
			return "", fmt.Errorf("%w: killed after %v (target unreadable: %v)", ErrTimeout, f.Timeout, readErr)
		}
		return "", fmt.Errorf("read target: %w", readErr)
	}

	if timedOut {
		return string(content), fmt.Errorf("%w: killed after %v", ErrTimeout, f.Timeout)
	}
	return string(content), nil
}

// EntitiesOfKind filters entities by kind
func (r *DetectResult) EntitiesOfKind(kind string) []DetectEntity {
	var filtered []DetectEntity
	for _, e := range r.Entities {
		if e.Kind == kind {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// HasEntity checks if any entity matches the given kind and text
func (r *DetectResult) HasEntity(kind, text string) bool {
	for _, e := range r.Entities {
		if e.Kind == kind && e.Text == text {
			return true
		}
	}
	return false
}

// HasEntityText checks if any entity (any kind) contains the given text
func (r *DetectResult) HasEntityText(text string) bool {
	for _, e := range r.Entities {
		if e.Text == text {
			return true
		}
	}
	return false
}

// EntityTexts returns all entity texts
func (r *DetectResult) EntityTexts() []string {
	texts := make([]string, len(r.Entities))
	for i, e := range r.Entities {
		texts[i] = e.Text
	}
	return texts
}

// EntityKinds returns the unique set of entity kinds found
func (r *DetectResult) EntityKinds() map[string]bool {
	kinds := make(map[string]bool)
	for _, e := range r.Entities {
		kinds[e.Kind] = true
	}
	return kinds
}

// RunTest executes a single PTY-based interaction test case
func (f *Framework) RunTest(testCase TestCase) TestResult {
	start := time.Now()
	result := TestResult{
		Name:   testCase.Name,
		Passed: false,
	}

	if err := f.BuildBinary(); err != nil {
		result.Error = fmt.Sprintf("failed to build binary: %v", err)
		result.Elapsed = time.Since(start)
		return result
	}

	tmpFile, err := os.CreateTemp("", "magonote-test-*.txt")
	if err != nil {
		result.Error = fmt.Sprintf("failed to create temp file: %v", err)
		result.Elapsed = time.Since(start)
		return result
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(testCase.Input); err != nil {
		result.Error = fmt.Sprintf("failed to write to temp file: %v", err)
		result.Elapsed = time.Since(start)
		return result
	}
	tmpFile.Close()

	args := append([]string{"-i", tmpFile.Name(), "--config", "NONE"}, testCase.Args...)

	cmd := exec.Command(f.BinaryPath, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 50, Cols: 200})
	if err != nil {
		result.Error = fmt.Sprintf("failed to start command: %v", err)
		result.Elapsed = time.Since(start)
		return result
	}
	defer ptmx.Close()

	time.Sleep(300 * time.Millisecond)

	if testCase.Keys != "" {
		_, err = ptmx.Write([]byte(testCase.Keys + "\n"))
		if err != nil {
			result.Error = fmt.Sprintf("failed to send keys: %v", err)
			result.Elapsed = time.Since(start)
			return result
		}
	}

	timeout := testCase.Timeout
	if timeout == 0 {
		timeout = f.Timeout
	}

	matchCh := make(chan string, 1)

	go func() {
		reader := bufio.NewReader(ptmx)
		var output strings.Builder

		for {
			b, err := reader.ReadByte()
			if err != nil {
				if err != io.EOF {
					// ignore read errors after process exits
				}
				matchCh <- output.String()
				return
			}

			output.WriteByte(b)

			if strings.Contains(output.String(), testCase.ExpectedOutput) {
				matchCh <- output.String()
				return
			}
		}
	}()

	select {
	case output := <-matchCh:
		if strings.Contains(output, testCase.ExpectedOutput) {
			result.Passed = true
		} else {
			result.Error = fmt.Sprintf("expected output not found in: %q", output)
		}
		result.Output = output

		// Clean up: wait for process to exit (short deadline to avoid hangs)
		waitDone := make(chan struct{})
		go func() { cmd.Wait(); close(waitDone) }()
		select {
		case <-waitDone:
		case <-time.After(2 * time.Second):
			cmd.Process.Kill()
		}
	case <-time.After(timeout):
		result.Error = "test timed out"
		cmd.Process.Kill()
		select {
		case output := <-matchCh:
			result.Output = output
		case <-time.After(500 * time.Millisecond):
		}
	}

	result.Elapsed = time.Since(start)
	return result
}

// RunTests executes multiple test cases
func (f *Framework) RunTests(testCases []TestCase) []TestResult {
	results := make([]TestResult, len(testCases))
	for i, testCase := range testCases {
		fmt.Printf("Running test: %s\n", testCase.Name)
		results[i] = f.RunTest(testCase)
		if results[i].Passed {
			fmt.Printf("✅ %s passed (%.2fs)\n", testCase.Name, results[i].Elapsed.Seconds())
		} else {
			fmt.Printf("❌ %s failed (%.2fs): %s\n", testCase.Name, results[i].Elapsed.Seconds(), results[i].Error)
		}
	}
	return results
}

// PrintSummary prints a summary of test results
func (f *Framework) PrintSummary(results []TestResult) {
	passed := 0
	total := len(results)

	fmt.Println("\n=== Test Summary ===")
	for _, result := range results {
		if result.Passed {
			passed++
			fmt.Printf("✅ %s\n", result.Name)
		} else {
			fmt.Printf("❌ %s: %s\n", result.Name, result.Error)
		}
	}

	fmt.Printf("\nTotal: %d, Passed: %d, Failed: %d\n", total, passed, total-passed)
	if passed == total {
		fmt.Println("🎉 All tests passed!")
	}
}
