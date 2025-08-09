# PS1Parser

A Go package for parsing zsh PS1 (prompt) strings and matching them against terminal output to find prompt positions.

## Features

- **Complete PS1 Parsing**: Supports all major zsh prompt escape sequences (`%n`, `%m`, `%~`, etc.)
- **Color Sequence Handling**: Parses `%{...%}` color escape sequences
- **Conditional Expressions**: Handles `%(test.true.false)` conditional prompts
- **Command Substitution**: Recognizes `$(command)` patterns
- **Flexible Matching**: Find prompts in terminal output with various options
- **Position Tracking**: Returns line and column positions of matched prompts

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    
    "your-repo/pkg/ps1parser"
)

func main() {
    // Define your zsh PS1 pattern
    ps1 := "%n@%m:%~ $ "
    
    // Terminal output containing prompts
    terminalOutput := `user@hostname:~/project $ ls
file1.txt  file2.txt

user@hostname:~/project $ pwd
/home/user/project

user@hostname:~/project $ `

    // Find all prompts
    matches, err := ps1parser.FindPrompts(ps1, terminalOutput)
    if err != nil {
        log.Fatal(err)
    }

    for i, match := range matches {
        fmt.Printf("Prompt %d: %s\n", i+1, match.Position)
        fmt.Printf("Content: %q\n", match.Matched)
    }
}
```

## Supported PS1 Elements

### Basic Escape Sequences
- `%n` - Username
- `%m` - Hostname (short)
- `%M` - Hostname (full)
- `%~` - Current directory with `~` substitution
- `%d`, `%/` - Current directory
- `%c`, `%.` - Current directory tail
- `%#` - Privilege indicator (`#` for root, `%` for user)
- `%?` - Exit status of last command
- `%h`, `%!` - History number

### Time and Date
- `%T` - Time (24-hour format)
- `%t`, `%@` - Time (12-hour format)
- `%*` - Time with seconds
- `%D` - Date (YY-MM-DD format)
- `%W` - Date (MM/DD/YY format)

### Color Sequences
- `%{...%}` - Non-printing escape sequences (colors, etc.)
- `%F{color}...%f` - Foreground color
- `%K{color}...%k` - Background color
- `%B...%b` - Bold text

### Conditional Expressions
- `%(test.true-text.false-text)` - Conditional prompt parts
- Common tests: `?` (exit status), `!` (privilege), `l` (line length)

### Command Substitution
- `$(command)` - Output of shell commands (e.g., `$(git_prompt_info)`)

## API Reference

### High-Level Functions

```go
// Find prompts with default options
func FindPrompts(ps1 string, terminalOutput string) ([]MatchResult, error)

// Find prompts with strict matching
func FindPromptsStrict(ps1 string, terminalOutput string) ([]MatchResult, error)

// Validate a PS1 string
func ValidatePS1(ps1 string) error

// Analyze PS1 structure
func AnalyzePS1(ps1 string) (*ParsedPS1, error)
```

### Detailed Control

```go
// Create parser with options
parser := ps1parser.NewParser(ps1parser.ParserOptions{
    StrictMode: true,
})

// Parse PS1 string
parsed, err := parser.Parse(ps1String)

// Create matcher with custom options
matcher, err := ps1parser.NewMatcher(parsed, ps1parser.MatchOptions{
    IgnoreColors:  true,
    CaseSensitive: false,
    MaxLineSpan:   3,
})

// Find matches
matches, err := matcher.Match(terminalOutput)
```

### Match Options

```go
type MatchOptions struct {
    IgnoreColors    bool // Ignore ANSI color codes
    IgnoreSpacing   bool // Ignore extra whitespace
    CaseSensitive   bool // Case-sensitive matching
    MaxLineSpan     int  // Max lines a prompt can span
    TimeoutPatterns bool // Match timeout patterns like "took 5m30s"
}
```

## Examples

### Basic Username@Hostname Prompt

```go
ps1 := "%n@%m:%~ $ "
text := "user@myhost:~/project $ ls"

matches, _ := ps1parser.FindPrompts(ps1, text)
// Returns: username="user", hostname="myhost", current_dir="~/project"
```

### Colored Prompt with Git Info

```go
ps1 := "%{%}%n%{%}@%m:%~ $(git_prompt_info)$ "
text := "user@host:~/repo main ✓ $ git status"

matches, _ := ps1parser.FindPrompts(ps1, text)
// Handles color sequences and git command output
```

### Conditional Prompt (Success/Error)

```go
ps1 := "%(?:%{%}✓ :%{%}✗ )%n:%~ $ "
text1 := "✓ user:~/work $ echo hello"
text2 := "✗ user:~/work $ false"

// Both will match despite different conditional branches
```

### Multi-line Prompt

```go
ps1 := "%n in %~ via 🐍 v3.13.5\n>> "
text := `user in ~/project via 🐍 v3.13.5
>> python script.py`

matches, _ := ps1parser.FindPrompts(ps1, text)
// Handles multi-line prompts correctly
```

## Real-World Examples

The parser handles complex real-world prompts from popular themes:

```go
// Oh-My-Zsh robbyrussell theme
ps1 := "%(?:%{%}➜ :%{%}➜ ) %{%}%c%{%} $(git_prompt_info)"

// Agnoster-style theme
ps1 := "%n@%m %{%}%~%{%} %{%}$(git_prompt_info)%{%} %# "

// Powerlevel10k-style
ps1 := "%{%}%n%{%} in %{%}🌐 %m%{%} in %{%}%~%{%} on %{%}$(git_branch)%{%}"
```

## Testing

Run the test suite:

```bash
go test ./pkg/ps1parser/...
```

The package includes comprehensive tests for:
- All PS1 escape sequences
- Color sequence parsing
- Conditional expressions
- Command substitutions
- Real-world prompt examples
- Edge cases and error conditions

## Error Handling

The parser provides detailed error messages for invalid PS1 strings:

```go
err := ps1parser.ValidatePS1("%{unclosed color")
// Returns: "parse error at position 20: unclosed color sequence"
```

## Performance

The parser is designed to be efficient for typical terminal output:
- Regex compilation is cached per pattern
- Memory usage scales with prompt complexity, not text length
- Optimized for common prompt patterns

## Limitations

- Git commands like `$(git_prompt_info)` are treated as generic command patterns
- Very complex nested conditionals may not parse perfectly
- Performance may degrade with extremely long terminal output
- Some advanced zsh features are not yet supported

## Contributing

Contributions welcome! Please ensure:
- New features include tests
- All tests pass
- Code follows Go best practices
- Comments use English as requested