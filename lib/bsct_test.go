package lib

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInteractiveBisector_SingleBadLine(t *testing.T) {
	lines := []string{"good1", "good2", "bad"}
	bisector := NewInteractiveBisector(lines, 0, 2, false)

	// Simulate user input: mark middle line as good
	input := "g\n"
	r := strings.NewReader(input)
	bisector.reader = bufio.NewReader(r)

	result, err := bisector.Bisect()
	require.NoError(t, err)
	assert.Equal(t, 3, result.BadLineNumber)
	assert.Equal(t, "bad", result.BadLineContent)
	assert.Equal(t, 1, result.StepsTaken)
}

func TestInteractiveBisector_MultipleBadLines(t *testing.T) {
	lines := []string{"good1", "good2", "bad1", "bad2", "bad3"}
	bisector := NewInteractiveBisector(lines, 0, 4, false)

	// Simulate: line 3 (idx 2) -> good, line 4 (idx 3) -> bad
	// This finds that bad2 (line 4) is the first bad one from the given test input
	input := "g\nb\n"
	r := strings.NewReader(input)
	bisector.reader = bufio.NewReader(r)

	result, err := bisector.Bisect()
	require.NoError(t, err)
	assert.Equal(t, 4, result.BadLineNumber)
	assert.Equal(t, "bad2", result.BadLineContent)
	assert.Equal(t, 2, result.StepsTaken)
}

func TestInteractiveBisector_InvalidInputRetry(t *testing.T) {
	lines := []string{"good1", "good2", "bad"}
	bisector := NewInteractiveBisector(lines, 0, 2, false)

	// Simulate: invalid input, then good
	input := "invalid\ng\n"
	r := strings.NewReader(input)
	bisector.reader = bufio.NewReader(r)

	result, err := bisector.Bisect()
	require.NoError(t, err)
	assert.Equal(t, 3, result.BadLineNumber)
	assert.Equal(t, 1, result.StepsTaken) // Invalid input doesn't count
}

func TestInteractiveBisector_AlternativeInputFormats(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"lowercase g", "g\n"},
		{"uppercase G", "G\n"},
		{"full good", "good\n"},
		{"full GOOD", "GOOD\n"},
		{"lowercase b", "b\n"},
		{"uppercase B", "B\n"},
		{"full bad", "bad\n"},
		{"full BAD", "BAD\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lines := []string{"good1", "good2", "test"}
			bisector := NewInteractiveBisector(lines, 0, 2, false)

			r := strings.NewReader(tc.input)
			bisector.reader = bufio.NewReader(r)

			result, err := bisector.Bisect()
			require.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}

func TestAutomaticBisector_WithTestCommand(t *testing.T) {
	lines := []string{"line1", "line2", "ERROR", "line4"}

	// Create a test script that fails if file contains "ERROR"
	script := `#!/bin/bash
if grep -q "ERROR" "$1"; then
  exit 1
fi
exit 0
`
	tmpScript, err := os.CreateTemp("", "test-*.sh")
	require.NoError(t, err)
	defer os.Remove(tmpScript.Name())

	_, err = tmpScript.WriteString(script)
	require.NoError(t, err)
	tmpScript.Close()

	err = os.Chmod(tmpScript.Name(), 0755)
	require.NoError(t, err)

	bisector := NewAutomaticBisector(lines, 0, 3, tmpScript.Name())

	result, err := bisector.Bisect()
	require.NoError(t, err)
	assert.Equal(t, 3, result.BadLineNumber)
	assert.Equal(t, "ERROR", result.BadLineContent)
	assert.Greater(t, result.StepsTaken, 0)
}

func TestAutomaticBisector_AllLinesPass(t *testing.T) {
	lines := []string{"good1", "good2", "good3"}

	// Create a test script that always passes
	script := `#!/bin/bash
exit 0
`
	tmpScript, err := os.CreateTemp("", "test-*.sh")
	require.NoError(t, err)
	defer os.Remove(tmpScript.Name())

	_, err = tmpScript.WriteString(script)
	require.NoError(t, err)
	tmpScript.Close()

	err = os.Chmod(tmpScript.Name(), 0755)
	require.NoError(t, err)

	bisector := NewAutomaticBisector(lines, 0, 2, tmpScript.Name())

	result, err := bisector.Bisect()
	require.NoError(t, err)
	// Should still identify the last line as the transition point
	assert.Equal(t, 3, result.BadLineNumber)
	assert.Equal(t, "good3", result.BadLineContent)
}

func TestAutomaticBisector_CommandExecutionCount(t *testing.T) {
	lines := make([]string, 16)
	for i := 0; i < 16; i++ {
		if i < 8 {
			lines[i] = "good"
		} else {
			lines[i] = "bad"
		}
	}

	// Create a test script that fails on "bad"
	script := `#!/bin/bash
if grep -q "bad" "$1"; then
  exit 1
fi
exit 0
`
	tmpScript, err := os.CreateTemp("", "test-*.sh")
	require.NoError(t, err)
	defer os.Remove(tmpScript.Name())

	_, err = tmpScript.WriteString(script)
	require.NoError(t, err)
	tmpScript.Close()

	err = os.Chmod(tmpScript.Name(), 0755)
	require.NoError(t, err)

	bisector := NewAutomaticBisector(lines, 0, 15, tmpScript.Name())

	result, err := bisector.Bisect()
	require.NoError(t, err)
	assert.Equal(t, 9, result.BadLineNumber)
	// Binary search should take log2(15) ≈ 4 steps
	assert.LessOrEqual(t, result.StepsTaken, 4)
}

func TestBisectionBoundaries(t *testing.T) {
	testCases := []struct {
		name      string
		lines     []string
		goodIdx   int
		badIdx    int
		expectBad int // 1-indexed
	}{
		{
			name:      "adjacent lines",
			lines:     []string{"good", "bad"},
			goodIdx:   0,
			badIdx:    1,
			expectBad: 2,
		},
		{
			name:      "start from middle",
			lines:     []string{"skip1", "skip2", "good", "bad1", "bad2"},
			goodIdx:   2,
			badIdx:    4,
			expectBad: 4, // First bad after good
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bisector := NewInteractiveBisector(tc.lines, tc.goodIdx, tc.badIdx, false)

			// For adjacent lines, no input needed
			if tc.badIdx-tc.goodIdx == 1 {
				result, err := bisector.Bisect()
				require.NoError(t, err)
				assert.Equal(t, tc.expectBad, result.BadLineNumber)
			} else {
				// Provide enough "b" responses to always go left
				input := strings.Repeat("b\n", 10)
				r := strings.NewReader(input)
				bisector.reader = bufio.NewReader(r)

				result, err := bisector.Bisect()
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestAutomaticBisector_FileContent(t *testing.T) {
	lines := []string{"line1", "line2", "line3", "line4"}

	// Create a test script that checks if line3 is present
	script := `#!/bin/bash
if grep -q "line3" "$1"; then
  exit 1
fi
exit 0
`
	tmpScript, err := os.CreateTemp("", "test-*.sh")
	require.NoError(t, err)
	defer os.Remove(tmpScript.Name())

	_, err = tmpScript.WriteString(script)
	require.NoError(t, err)
	tmpScript.Close()

	err = os.Chmod(tmpScript.Name(), 0755)
	require.NoError(t, err)

	bisector := NewAutomaticBisector(lines, 0, 3, tmpScript.Name())

	result, err := bisector.Bisect()
	require.NoError(t, err)
	assert.Equal(t, 3, result.BadLineNumber)
	assert.Equal(t, "line3", result.BadLineContent)
}

func TestResultStepsCounting(t *testing.T) {
	lines := make([]string, 8)
	for i := range lines {
		lines[i] = "line"
	}

	bisector := NewInteractiveBisector(lines, 0, 7, false)

	// Simulate binary search path: b, b
	// Start: 0-7, test 3 (bad) -> 0-3, test 1 (bad) -> 0-1 (done, 2 steps)
	input := "b\nb\n"
	r := strings.NewReader(input)
	bisector.reader = bufio.NewReader(r)

	result, err := bisector.Bisect()
	require.NoError(t, err)
	assert.Equal(t, 2, result.StepsTaken)
}

func TestAutomaticBisector_LinePlaceholder(t *testing.T) {
	lines := []string{"good line", "another good", "ERROR: bad line", "more bad"}

	// Create a test script that checks if the line content contains "ERROR"
	script := `#!/bin/bash
if echo "$1" | grep -q "ERROR"; then
  exit 1
fi
exit 0
`
	tmpScript, err := os.CreateTemp("", "test-*.sh")
	require.NoError(t, err)
	defer os.Remove(tmpScript.Name())

	_, err = tmpScript.WriteString(script)
	require.NoError(t, err)
	tmpScript.Close()

	err = os.Chmod(tmpScript.Name(), 0755)
	require.NoError(t, err)

	// Use {line} placeholder to pass line content to the script
	bisector := NewAutomaticBisector(lines, 0, 3, tmpScript.Name()+" {line}")

	result, err := bisector.Bisect()
	require.NoError(t, err)
	assert.Equal(t, 3, result.BadLineNumber)
	assert.Equal(t, "ERROR: bad line", result.BadLineContent)
}

func TestAutomaticBisector_FilePlaceholder(t *testing.T) {
	lines := []string{"line1", "line2", "ERROR", "line4"}

	// Create a test script using explicit {file} placeholder
	script := `#!/bin/bash
if grep -q "ERROR" "$1"; then
  exit 1
fi
exit 0
`
	tmpScript, err := os.CreateTemp("", "test-*.sh")
	require.NoError(t, err)
	defer os.Remove(tmpScript.Name())

	_, err = tmpScript.WriteString(script)
	require.NoError(t, err)
	tmpScript.Close()

	err = os.Chmod(tmpScript.Name(), 0755)
	require.NoError(t, err)

	// Use {file} placeholder explicitly
	bisector := NewAutomaticBisector(lines, 0, 3, tmpScript.Name()+" {file}")

	result, err := bisector.Bisect()
	require.NoError(t, err)
	assert.Equal(t, 3, result.BadLineNumber)
	assert.Equal(t, "ERROR", result.BadLineContent)
}

func TestAutomaticBisector_BracesPlaceholder(t *testing.T) {
	lines := []string{"line1", "line2", "ERROR", "line4"}

	// Create a test script using {} placeholder
	script := `#!/bin/bash
if grep -q "ERROR" "$1"; then
  exit 1
fi
exit 0
`
	tmpScript, err := os.CreateTemp("", "test-*.sh")
	require.NoError(t, err)
	defer os.Remove(tmpScript.Name())

	_, err = tmpScript.WriteString(script)
	require.NoError(t, err)
	tmpScript.Close()

	err = os.Chmod(tmpScript.Name(), 0755)
	require.NoError(t, err)

	// Use {} placeholder
	bisector := NewAutomaticBisector(lines, 0, 3, tmpScript.Name()+" {}")

	result, err := bisector.Bisect()
	require.NoError(t, err)
	assert.Equal(t, 3, result.BadLineNumber)
	assert.Equal(t, "ERROR", result.BadLineContent)
}

func TestAutomaticBisector_MixedPlaceholders(t *testing.T) {
	lines := []string{"good", "still good", "BAD_LINE", "also bad"}

	// Create a test script that uses both file and line content
	script := `#!/bin/bash
# Check if line contains "BAD" AND if file contains it
if echo "$2" | grep -q "BAD"; then
  if grep -q "BAD" "$1"; then
    exit 1
  fi
fi
exit 0
`
	tmpScript, err := os.CreateTemp("", "test-*.sh")
	require.NoError(t, err)
	defer os.Remove(tmpScript.Name())

	_, err = tmpScript.WriteString(script)
	require.NoError(t, err)
	tmpScript.Close()

	err = os.Chmod(tmpScript.Name(), 0755)
	require.NoError(t, err)

	// Use both {file} and {line} placeholders
	bisector := NewAutomaticBisector(lines, 0, 3, tmpScript.Name()+" {file} {line}")

	result, err := bisector.Bisect()
	require.NoError(t, err)
	assert.Equal(t, 3, result.BadLineNumber)
	assert.Equal(t, "BAD_LINE", result.BadLineContent)
}

// TestMain ensures test scripts are executable
func TestMain(m *testing.M) {
	// Check if we can execute shell scripts
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Run(); err != nil {
		panic("Cannot execute shell commands in test environment")
	}

	os.Exit(m.Run())
}
