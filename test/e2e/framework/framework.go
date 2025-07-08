package framework

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/creack/pty"
)

// findProjectRoot searches for the project root directory containing go.mod
func findProjectRoot(startDir string) string {
	dir := startDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// Check if this go.mod declares the main module (not just requires it)
			content, err := os.ReadFile(goModPath)
			if err == nil && strings.HasPrefix(strings.TrimSpace(string(content)), "module github.com/Hanaasagi/magonote\n") {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root directory
		}
		dir = parent
	}
	return ""
}

// Framework provides utilities for running e2e tests
type Framework struct {
	BinaryPath string
	Timeout    time.Duration
}

// TestCase represents a single e2e test case
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
		return nil // Already set
	}

	// Find the project root
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Navigate to project root (find the directory containing go.mod for main project)
	projectRoot := findProjectRoot(wd)
	if projectRoot == "" {
		return fmt.Errorf("could not find project root directory from %s", wd)
	}

	buildDir := filepath.Join(projectRoot, "build")
	binaryPath := filepath.Join(buildDir, "magonote")

	// Ensure build directory exists
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Build the binary
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/magonote")
	cmd.Dir = projectRoot

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to build binary: %w, output: %s", err, string(output))
	}

	f.BinaryPath = binaryPath
	return nil
}

// RunTest executes a single test case
func (f *Framework) RunTest(testCase TestCase) TestResult {
	start := time.Now()
	result := TestResult{
		Name:   testCase.Name,
		Passed: false,
	}

	// Ensure binary is built
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

	// Use pty to start command
	ptmx, err := pty.Start(cmd)
	if err != nil {
		result.Error = fmt.Sprintf("failed to start command: %v", err)
		result.Elapsed = time.Since(start)
		return result
	}
	defer ptmx.Close()

	// Wait for program initialization
	time.Sleep(200 * time.Millisecond)

	// Send keys
	if testCase.Keys != "" {
		_, err = ptmx.Write([]byte(testCase.Keys + "\n"))
		if err != nil {
			result.Error = fmt.Sprintf("failed to send keys: %v", err)
			result.Elapsed = time.Since(start)
			return result
		}
	}

	// Set timeout
	timeout := testCase.Timeout
	if timeout == 0 {
		timeout = f.Timeout
	}
	timeoutCh := time.After(timeout)

	// Create channel for match result
	matchCh := make(chan bool)
	outputCh := make(chan string)

	// Read output in a separate goroutine
	go func() {
		reader := bufio.NewReader(ptmx)
		var output strings.Builder

		for {
			select {
			case <-timeoutCh:
				outputCh <- output.String()
				return
			default:
				b, err := reader.ReadByte()
				if err != nil {
					if err != io.EOF {
						result.Error = fmt.Sprintf("error reading output: %v", err)
					}
					outputCh <- output.String()
					return
				}

				output.WriteByte(b)

				// Check if output contains expected string
				if strings.Contains(output.String(), testCase.ExpectedOutput) {
					matchCh <- true
					outputCh <- output.String()
					return
				}
			}
		}
	}()

	// Wait for match result or timeout
	select {
	case <-matchCh:
		result.Passed = true
		result.Output = <-outputCh
	case <-timeoutCh:
		result.Error = "test timed out"
		result.Output = <-outputCh
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
