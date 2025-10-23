# bsct - Bisect Text Lines

A CLI tool that interactively bisects input lines to find the first bad line, similar to `git bisect` but for text files or stdin.

## Installation

```bash
go install github.com/knpwrs/bsct@latest
```

Or build from source:

```bash
git clone https://github.com/knpwrs/bsct
cd bsct
go build
```

## Usage

### Basic Interactive Mode

Read from a file and interactively determine which line is bad:

```bash
bsct input.txt
```

Or pipe from stdin:

```bash
cat logfile.txt | bsct
```

**Note:** When using stdin for input data, `bsct` automatically reads your interactive responses from `/dev/tty` instead of stdin. This allows you to pipe data in while still answering prompts interactively.

The tool displays each test line with context (the line before and after) for easy identification. The line being tested is highlighted with a colored line number, while context lines appear faded:

```
Step 1: Testing line 50 of 100

  49 | Previous line content (faded)
  50 | Line being tested here (highlighted)
  51 | Next line content (faded)

Is this line good or bad? [g/b]:
```

Type `g` (or `good`) if the line is good, `b` (or `bad`) if the line is bad.

### Automatic Mode with Test Command

Use the `--test` flag to automatically determine good/bad lines by running a command:

```bash
bsct input.txt --test "./validate.sh"
```

The test command should exit with code 0 if the test passes (good) or non-zero if it fails (bad).

#### Placeholders

The test command supports these placeholders:

- **`{file}` or `{}`** - Replaced with a temporary file path containing lines 1 through the test line
- **`{line}`** - Replaced with the content of the line being tested

**Examples:**

Test using the file content:

```bash
bsct input.txt --test "grep -q ERROR {file} && exit 1 || exit 0"
```

Test using just the line content:

```bash
bsct commits.txt --test 'echo {line} | grep -q "BREAKING:" && exit 1 || exit 0'
```

Test with a script file:

```bash
#!/bin/bash
# Check if the file contains an error
if grep -q "ERROR" "$1"; then
  exit 1  # Bad
fi
exit 0  # Good
```

```bash
bsct input.txt --test "./validate.sh {file}"
# or without placeholder for backward compatibility
bsct input.txt --test "./validate.sh"
```

### Pattern-Based Boundaries

Use `--good` and `--bad` flags to automatically find starting boundaries:

```bash
bsct input.txt --good "SUCCESS" --bad "FATAL"
```

This finds the first line containing "SUCCESS" as the known good line and the first line containing "FATAL" as the known bad line, then bisects between them.

### Combining Flags

```bash
bsct logs.txt --good "START" --bad "ERROR" --test "./check.sh"
```

## Examples

### Example 1: Finding a Bad Commit in Git Log

```bash
git log --oneline > commits.txt
bsct commits.txt --test "git checkout \$(head -1 \$1 | cut -d' ' -f1) && make test"
```

### Example 2: Finding When a Bug Was Introduced

Given a file with timestamped log entries:

```bash
cat application.log | bsct --bad "NullPointerException"
```

### Example 3: Binary Search Through Large Files

```bash
bsct large_dataset.csv --test "python validate_data.py"
```

## How It Works

The tool uses binary search to efficiently find the first "bad" line:

1. Starts with the assumption that the first line is good and the last line is bad (or uses `--good`/`--bad` patterns)
2. Tests the middle line
3. If the middle line is good, searches the upper half
4. If the middle line is bad, searches the lower half
5. Repeats until the first bad line is found

This takes O(log n) steps instead of O(n) for linear search.

## Output

When bisection completes, bsct displays:

```
=== Bisection Complete ===
First bad line: 47
Content: ERROR: Database connection failed
Steps taken: 6
```

## Flags

- `--good <pattern>`: Content pattern to identify a known good line
- `--bad <pattern>`: Content pattern to identify a known bad line
- `--test <command>`: Command to run for automatic testing (exit 0 = good, non-zero = bad)

## Testing

Run the test suite:

```bash
go test ./lib/... -v
```

## License

CC0 / UNLICENSE
