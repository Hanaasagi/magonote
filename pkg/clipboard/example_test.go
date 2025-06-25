package clipboard_test

import (
	"fmt"
	"log"
	"os"

	"github.com/Hanaasagi/magonote/pkg/clipboard"
)

func ExampleNew() {
	// Create a new clipboard with default settings (all targets enabled)
	c := clipboard.New()

	// Copy text to all available clipboard targets
	err := c.Copy("Hello, World!")
	if err != nil {
		log.Printf("Copy failed: %v", err)
	}

	fmt.Println("Text copied to clipboard")
	// Output: Text copied to clipboard
}

func ExampleNew_withOptions() {
	// Create clipboard with specific options
	c := clipboard.New(
		clipboard.WithTmux(true),        // Enable tmux buffer
		clipboard.WithSystem(false),     // Disable system clipboard
		clipboard.WithOSC52(true),       // Enable OSC52 terminal copying
		clipboard.WithOutput(os.Stderr), // Set output for OSC52
	)

	err := c.Copy("Configured clipboard")
	if err != nil {
		log.Printf("Copy failed: %v", err)
	}

	fmt.Println("Text copied with custom configuration")
	// Output: Text copied with custom configuration
}

func ExampleCopy() {
	// Quick way to copy text using default settings
	err := clipboard.Copy("Quick copy!")
	if err != nil {
		log.Printf("Copy failed: %v", err)
	}

	fmt.Println("Quick copy completed")
	// Output: Quick copy completed
}

func ExampleCopyToTmux() {
	// Copy only to tmux buffer
	err := clipboard.CopyToTmux("Tmux only")
	if err != nil {
		log.Printf("Tmux copy failed: %v", err)
	}

	fmt.Println("Copied to tmux buffer")
	// Output: Copied to tmux buffer
}

func ExampleCopyToSystem() {
	// Copy only to system clipboard
	err := clipboard.CopyToSystem("System only")
	if err != nil {
		log.Printf("System copy failed: %v", err)
	}

	fmt.Println("Copied to system clipboard")
	// Output: Copied to system clipboard
}

func ExampleCopyWithOSC52() {
	// Copy using OSC52 terminal escape sequence
	err := clipboard.CopyWithOSC52("Remote copy via OSC52")
	if err != nil {
		log.Printf("OSC52 copy failed: %v", err)
	}

	fmt.Println("Copied via OSC52")
	// Output: Copied via OSC52
}

func ExampleOSC52Writer() {
	// Create an OSC52 writer for custom output
	writer := clipboard.NewOSC52Writer(os.Stderr)

	err := writer.Write("Custom OSC52 output")
	if err != nil {
		log.Printf("OSC52 write failed: %v", err)
	}

	fmt.Println("OSC52 sequence written")
	// Output: OSC52 sequence written
}

func ExampleTmuxWriter() {
	// Create a tmux-only writer
	writer := clipboard.NewTmuxWriter()

	err := writer.Write("Tmux buffer content")
	if err != nil {
		log.Printf("Tmux write failed: %v", err)
	}

	fmt.Println("Written to tmux buffer")
	// Output: Written to tmux buffer
}

func ExampleSystemWriter() {
	// Create a system-only writer
	writer := clipboard.NewSystemWriter()

	err := writer.Write("System clipboard content")
	if err != nil {
		log.Printf("System write failed: %v", err)
	}

	fmt.Println("Written to system clipboard")
	// Output: Written to system clipboard
}

func ExampleAvailable() {
	// Check which clipboard targets are available
	available := clipboard.Available()

	fmt.Printf("Tmux available: %v\n", available["tmux"])
	fmt.Printf("System available: %v\n", available["system"])
	fmt.Printf("OSC52 available: %v\n", available["osc52"])

	// Output will vary based on environment
}

func ExampleIsTmuxSession() {
	// Check if running inside tmux
	if clipboard.IsTmuxSession() {
		fmt.Println("Running inside tmux")
	} else {
		fmt.Println("Not running inside tmux")
	}

	// Output will vary based on environment
}

func ExampleHasSystemClipboard() {
	// Check if system clipboard tools are available
	if clipboard.HasSystemClipboard() {
		fmt.Println("System clipboard tools available")
	} else {
		fmt.Println("No system clipboard tools found")
	}

	// Output will vary based on system
}

// Example: Remote SSH session clipboard integration
func Example_remoteSSH() {
	// In a remote SSH session, you typically want OSC52
	// which works through terminal escape sequences

	// Check if we're in a remote session (simplified check)
	isRemote := os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != ""

	var c *clipboard.Clipboard
	if isRemote {
		// For remote sessions, prefer OSC52 over system tools
		c = clipboard.New(
			clipboard.WithOSC52(true),
			clipboard.WithSystem(false),
			clipboard.WithTmux(clipboard.IsTmuxSession()),
		)
	} else {
		// For local sessions, use all available methods
		c = clipboard.New()
	}

	err := c.Copy("Remote clipboard content")
	if err != nil {
		log.Printf("Remote copy failed: %v", err)
	}

	fmt.Println("Configured for remote/local session")
	// Output: Configured for remote/local session
}

// Example: Custom writer implementing the Writer interface
func ExampleWriter() {
	// You can implement custom clipboard writers
	type CustomWriter struct{}

	writeFunc := func(w *CustomWriter) func(string) error {
		return func(text string) error {
			fmt.Printf("Custom writer received: %s\n", text)
			return nil
		}
	}

	// Use the custom writer
	writer := &CustomWriter{}
	writeMethod := writeFunc(writer)
	err := writeMethod("Custom clipboard implementation")
	if err != nil {
		log.Printf("Custom write failed: %v", err)
	}

	// Output: Custom writer received: Custom clipboard implementation
}
