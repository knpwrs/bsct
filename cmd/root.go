package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/knpwrs/bsct/lib"
	"github.com/spf13/cobra"
)

var (
	goodPattern   string
	badPattern    string
	testCommand   string
	beforeCommand string
	afterCommand  string
)

var rootCmd = &cobra.Command{
	Use:   "bsct [file]",
	Short: "Bisect input lines to find the first bad line",
	Long: `bsct is a CLI tool that interactively bisects input lines to find the first bad line.
It works similar to git bisect but for text files or stdin.

You can provide input via:
  - A file path argument
  - stdin (pipe or redirect)

By default, the first line is assumed good and the last line is assumed bad.
Use --good and --bad flags to specify content patterns for automatic boundary detection.
Use --test to run a command automatically instead of interactive prompts.

Placeholders (supported in --test, --before, and --after):
  {file} or {} - replaced with temp file path (lines 1 through test line)
  {line} - replaced with the current line content being tested

Hooks:
  --before - runs before each test (useful for setup steps)
  --after - runs after each test (useful for cleanup steps)`,
	Args: cobra.MaximumNArgs(1),
	RunE: run,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVar(&goodPattern, "good", "", "Content pattern to identify a known good line")
	rootCmd.Flags().StringVar(&badPattern, "bad", "", "Content pattern to identify a known bad line")
	rootCmd.Flags().StringVar(&testCommand, "test", "", "Command to run for automatic testing (exit 0 = good, non-zero = bad). Supports {file}, {}, and {line} placeholders")
	rootCmd.Flags().StringVar(&beforeCommand, "before", "", "Command to run before each test (useful for setup). Supports {file}, {}, and {line} placeholders")
	rootCmd.Flags().StringVar(&afterCommand, "after", "", "Command to run after each test (useful for cleanup). Supports {file}, {}, and {line} placeholders")
}

func run(cmd *cobra.Command, args []string) error {
	// Read input lines
	lines, usingStdin, err := readInput(args)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	if len(lines) == 0 {
		return fmt.Errorf("no input lines provided")
	}

	// Find initial boundaries
	goodIdx, badIdx, err := findBoundaries(lines, goodPattern, badPattern)
	if err != nil {
		return err
	}

	// Create bisector
	var bisector lib.Bisector
	if testCommand != "" {
		bisector = lib.NewAutomaticBisector(lines, goodIdx, badIdx, testCommand, beforeCommand, afterCommand)
	} else {
		bisector = lib.NewInteractiveBisector(lines, goodIdx, badIdx, usingStdin)
	}

	// Run bisection
	result, err := bisector.Bisect()
	if err != nil {
		return err
	}

	// Print results
	const (
		colorReset  = "\033[0m"
		colorGreen  = "\033[32m"
		colorRed    = "\033[31m"
		colorFaded  = "\033[2m"
		colorBold   = "\033[1m"
		separator   = "═════════════════════════════════════════════════════════════"
	)

	fmt.Println()
	fmt.Printf("%s%s%s\n", colorGreen, separator, colorReset)
	fmt.Printf("%s%s✓ Bisection Complete%s\n", colorBold, colorGreen, colorReset)
	fmt.Printf("%s%s%s\n", colorGreen, separator, colorReset)
	fmt.Println()
	fmt.Printf("The first bad line is %s%s%d%s\n", colorBold, colorRed, result.BadLineNumber, colorReset)

	// Display the bad line with context
	badLineIdx := result.BadLineNumber - 1 // Convert to 0-indexed
	displayResultContext(lines, badLineIdx)

	fmt.Printf("%sSteps taken:%s %d\n", colorBold, colorReset, result.StepsTaken)
	fmt.Println()

	return nil
}

func readInput(args []string) ([]string, bool, error) {
	var scanner *bufio.Scanner
	usingStdin := false

	if len(args) > 0 {
		// Read from file
		file, err := os.Open(args[0])
		if err != nil {
			return nil, false, err
		}
		defer file.Close()
		scanner = bufio.NewScanner(file)
	} else {
		// Check if stdin is from pipe or redirect
		stat, err := os.Stdin.Stat()
		if err != nil {
			return nil, false, err
		}
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return nil, false, fmt.Errorf("no input provided: specify a file argument or pipe/redirect stdin")
		}
		scanner = bufio.NewScanner(os.Stdin)
		usingStdin = true
	}

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, false, err
	}

	return lines, usingStdin, nil
}

func findBoundaries(lines []string, goodPattern, badPattern string) (int, int, error) {
	goodIdx := 0
	badIdx := len(lines) - 1

	// Search for good pattern if provided
	if goodPattern != "" {
		found := false
		for i, line := range lines {
			if strings.Contains(line, goodPattern) {
				goodIdx = i
				found = true
				break
			}
		}
		if !found {
			return 0, 0, fmt.Errorf("good pattern %q not found in input", goodPattern)
		}
	}

	// Search for bad pattern if provided
	if badPattern != "" {
		found := false
		for i, line := range lines {
			if strings.Contains(line, badPattern) {
				badIdx = i
				found = true
				break
			}
		}
		if !found {
			return 0, 0, fmt.Errorf("bad pattern %q not found in input", badPattern)
		}
	}

	if goodIdx >= badIdx {
		return 0, 0, fmt.Errorf("good line (index %d) must come before bad line (index %d)", goodIdx, badIdx)
	}

	return goodIdx, badIdx, nil
}

func displayResultContext(lines []string, badIdx int) {
	const (
		colorReset = "\033[0m"
		colorRed   = "\033[31m"
		colorFaded = "\033[2m"
		colorBold  = "\033[1m"
	)

	fmt.Println()

	// Show line before (if exists)
	if badIdx > 0 {
		lineNum := badIdx // Line number is badIdx (0-indexed badIdx = line badIdx in 1-indexed)
		fmt.Printf("%s%4d | %s%s\n", colorFaded, lineNum, lines[badIdx-1], colorReset)
	}

	// Show the bad line (highlighted in red)
	lineNum := badIdx + 1 // Convert 0-indexed to 1-indexed for display
	fmt.Printf("%s%s%4d | %s%s%s\n", colorBold, colorRed, lineNum, lines[badIdx], colorReset, colorReset)

	// Show line after (if exists)
	if badIdx < len(lines)-1 {
		lineNum := badIdx + 2 // Line after the bad line
		fmt.Printf("%s%4d | %s%s\n", colorFaded, lineNum, lines[badIdx+1], colorReset)
	}

	fmt.Println()
}
