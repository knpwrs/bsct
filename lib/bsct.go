package lib

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Result contains the outcome of a bisection
type Result struct {
	BadLineNumber  int    // 1-indexed line number
	BadLineContent string // Content of the bad line
	StepsTaken     int    // Number of bisection steps
}

// Bisector defines the interface for bisection strategies
type Bisector interface {
	Bisect() (*Result, error)
}

// InteractiveBisector performs bisection with user prompts
type InteractiveBisector struct {
	lines    []string
	goodIdx  int
	badIdx   int
	steps    int
	reader   *bufio.Reader
	ttyFile  *os.File
}

// NewInteractiveBisector creates a new interactive bisector
func NewInteractiveBisector(lines []string, goodIdx, badIdx int, usingStdin bool) *InteractiveBisector {
	var reader *bufio.Reader
	var ttyFile *os.File

	if usingStdin {
		// When stdin is used for data, open /dev/tty for interactive prompts
		var err error
		ttyFile, err = os.Open("/dev/tty")
		if err != nil {
			// Fallback to stdin if /dev/tty can't be opened
			reader = bufio.NewReader(os.Stdin)
		} else {
			reader = bufio.NewReader(ttyFile)
		}
	} else {
		// Normal case: read from stdin
		reader = bufio.NewReader(os.Stdin)
	}

	return &InteractiveBisector{
		lines:   lines,
		goodIdx: goodIdx,
		badIdx:  badIdx,
		reader:  reader,
		ttyFile: ttyFile,
	}
}

// Bisect performs interactive bisection
func (b *InteractiveBisector) Bisect() (*Result, error) {
	const (
		colorReset = "\033[0m"
		colorGreen = "\033[32m"
		colorRed   = "\033[31m"
		colorBlue  = "\033[34m"
		colorBold  = "\033[1m"
		separator  = "─────────────────────────────────────────────────────────────"
	)

	// Ensure tty file is closed when we're done
	if b.ttyFile != nil {
		defer b.ttyFile.Close()
	}

	fmt.Printf("%s%sStarting bisection%s between lines %d and %d (%d lines total)\n",
		colorBold, colorBlue, colorReset, b.goodIdx+1, b.badIdx+1, len(b.lines))
	fmt.Println("Type 'g' or 'good' if the line is good, 'b' or 'bad' if the line is bad")
	fmt.Println()

	for b.badIdx-b.goodIdx > 1 {
		midIdx := b.goodIdx + (b.badIdx-b.goodIdx)/2
		b.steps++

		// Visual separator for each step
		fmt.Printf("%s%s%s\n", colorBlue, separator, colorReset)
		fmt.Printf("%s%sStep %d:%s Testing line %d of %d\n", colorBold, colorBlue, b.steps, colorReset, midIdx+1, len(b.lines))
		b.displayLineWithContext(midIdx)
		fmt.Print("Is this line good or bad? [g/b]: ")

		response, err := b.reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))

		switch response {
		case "g", "good":
			b.goodIdx = midIdx
			fmt.Printf("%s✓ Marked as good%s. Searching lines %d-%d\n", colorGreen, colorReset, b.goodIdx+1, b.badIdx+1)
		case "b", "bad":
			b.badIdx = midIdx
			fmt.Printf("%s✗ Marked as bad%s. Searching lines %d-%d\n", colorRed, colorReset, b.goodIdx+1, b.badIdx+1)
		default:
			fmt.Printf("%s⚠ Invalid input%s. Please enter 'g' (good) or 'b' (bad)\n", colorRed, colorReset)
			b.steps-- // Don't count invalid steps
		}
		fmt.Println()
	}

	return &Result{
		BadLineNumber:  b.badIdx + 1, // Convert to 1-indexed
		BadLineContent: b.lines[b.badIdx],
		StepsTaken:     b.steps,
	}, nil
}

// displayLineWithContext shows the line being tested with context lines above and below
func (b *InteractiveBisector) displayLineWithContext(idx int) {
	const (
		// ANSI color codes
		colorReset  = "\033[0m"
		colorFaded  = "\033[2m"      // Faded/dim text
		colorCyan   = "\033[36m"     // Cyan for line number being tested
		colorBold   = "\033[1m"      // Bold for emphasis
	)

	fmt.Println()

	// Show line before (if exists)
	if idx > 0 {
		lineNum := idx // 0-indexed
		fmt.Printf("%s%4d | %s%s\n", colorFaded, lineNum, b.lines[idx-1], colorReset)
	}

	// Show current line being tested (highlighted)
	lineNum := idx + 1 // 1-indexed for display
	fmt.Printf("%s%s%4d%s | %s%s\n", colorBold, colorCyan, lineNum, colorReset, b.lines[idx], colorReset)

	// Show line after (if exists)
	if idx < len(b.lines)-1 {
		lineNum := idx + 2 // 0-indexed + 2
		fmt.Printf("%s%4d | %s%s\n", colorFaded, lineNum, b.lines[idx+1], colorReset)
	}

	fmt.Println()
}

// AutomaticBisector performs bisection using a test command
type AutomaticBisector struct {
	lines       []string
	goodIdx     int
	badIdx      int
	steps       int
	testCommand string
}

// NewAutomaticBisector creates a new automatic bisector
func NewAutomaticBisector(lines []string, goodIdx, badIdx int, testCommand string) *AutomaticBisector {
	return &AutomaticBisector{
		lines:       lines,
		goodIdx:     goodIdx,
		badIdx:      badIdx,
		testCommand: testCommand,
	}
}

// Bisect performs automatic bisection using the test command
func (b *AutomaticBisector) Bisect() (*Result, error) {
	fmt.Printf("Starting automatic bisection between lines %d and %d (%d lines total)\n",
		b.goodIdx+1, b.badIdx+1, len(b.lines))
	fmt.Printf("Test command: %s\n", b.testCommand)
	fmt.Println()

	for b.badIdx-b.goodIdx > 1 {
		midIdx := b.goodIdx + (b.badIdx-b.goodIdx)/2
		b.steps++

		fmt.Printf("Step %d: Testing line %d of %d\n", b.steps, midIdx+1, len(b.lines))
		fmt.Printf("Line content: %s\n", b.lines[midIdx])

		// Create temporary file with content up to midIdx
		tmpFile, err := os.CreateTemp("", "bsct-*.txt")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		// Write lines from beginning through midIdx
		for i := 0; i <= midIdx; i++ {
			if _, err := tmpFile.WriteString(b.lines[i] + "\n"); err != nil {
				tmpFile.Close()
				return nil, fmt.Errorf("failed to write temp file: %w", err)
			}
		}
		tmpFile.Close()

		// Build command with placeholder substitution
		cmdStr := b.buildCommand(tmpPath, b.lines[midIdx])
		cmd := exec.Command("sh", "-c", cmdStr)
		err = cmd.Run()

		if err == nil {
			// Exit code 0 means good
			b.goodIdx = midIdx
			fmt.Printf("Test passed (good). Searching lines %d-%d\n\n", b.goodIdx+1, b.badIdx+1)
		} else {
			// Non-zero exit code means bad
			b.badIdx = midIdx
			fmt.Printf("Test failed (bad). Searching lines %d-%d\n\n", b.goodIdx+1, b.badIdx+1)
		}
	}

	return &Result{
		BadLineNumber:  b.badIdx + 1, // Convert to 1-indexed
		BadLineContent: b.lines[b.badIdx],
		StepsTaken:     b.steps,
	}, nil
}

// buildCommand constructs the command string with placeholder substitutions
// Supports:
//   {} or {file} - replaced with the temp file path
//   {line} - replaced with the current line content
func (b *AutomaticBisector) buildCommand(filePath, lineContent string) string {
	cmdStr := b.testCommand

	// Check if command contains placeholders
	hasPlaceholder := strings.Contains(cmdStr, "{}")
	hasFilePlaceholder := strings.Contains(cmdStr, "{file}")
	hasLinePlaceholder := strings.Contains(cmdStr, "{line}")

	// Replace {line} with the actual line content (properly quoted)
	if hasLinePlaceholder {
		quotedLine := strings.ReplaceAll(lineContent, "'", "'\\''")
		cmdStr = strings.ReplaceAll(cmdStr, "{line}", fmt.Sprintf("'%s'", quotedLine))
	}

	// Replace {} or {file} with the temp file path
	if hasPlaceholder {
		cmdStr = strings.ReplaceAll(cmdStr, "{}", filePath)
	}
	if hasFilePlaceholder {
		cmdStr = strings.ReplaceAll(cmdStr, "{file}", filePath)
	}

	// If no placeholders found, append file path as before (backward compatibility)
	if !hasPlaceholder && !hasFilePlaceholder && !hasLinePlaceholder {
		cmdStr = fmt.Sprintf("%s %s", cmdStr, filePath)
	}

	return cmdStr
}
