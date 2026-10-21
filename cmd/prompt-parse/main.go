// Command prompt-parse: clipboard-first PII scrubber.
//
// Typical flow (no app to open):
//  1. Copy logs from prod.
//  2. Run global hotkey -> `prompt-parse --clipboard --copy`
//  3. Paste the clean version into any LLM chat.
//
// Subcommands are flags for simplicity so Raycast / PowerToys / AHK
// can call one binary with args.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/idarciooliveira/prompt-parse/internal/redact"
)

func main() {
	var (
		useClipboard = flag.Bool("clipboard", false, "read input from clipboard instead of stdin/file args")
		copyBack     = flag.Bool("copy", false, "write cleaned text back to clipboard")
		showStats    = flag.Bool("stats", false, "print redaction stats to stderr")
		pattern      = flag.String("pattern", "", "extra custom regex (repeatable via comma-separated list)")
		maxLen       = flag.Int("max", 0, "truncate output to N chars (0 = no limit)")
	)
	flag.Parse()

	var input string
	var err error

	if *useClipboard {
		input, err = clipboard.ReadAll()
		if err != nil {
			fatal("read clipboard: %v (Linux needs xclip/xsel, WSL needs win32yank)", err)
		}
		if strings.TrimSpace(input) == "" {
			fatal("clipboard is empty — copy logs first")
		}
	} else if flag.NArg() > 0 {
		// File args: concatenate.
		var sb strings.Builder
		for _, f := range flag.Args() {
			b, err := os.ReadFile(f)
			if err != nil {
				fatal("read %s: %v", f, err)
			}
			sb.Write(b)
			sb.WriteString("\n")
		}
		input = sb.String()
	} else if hasStdin() {
		b, err := io.ReadAll(bufio.NewReader(os.Stdin))
		if err != nil {
			fatal("read stdin: %v", err)
		}
		input = string(b)
	} else {
		flag.Usage()
		fmt.Fprintln(os.Stderr, "\nExample: prompt-parse --clipboard --copy --stats")
		os.Exit(2)
	}

	var extras []string
	if *pattern != "" {
		extras = strings.Split(*pattern, ",")
	}
	res := redact.Clean(input, extras)
	out := res.Text
	if *maxLen > 0 && len(out) > *maxLen {
		out = out[:*maxLen] + fmt.Sprintf("\n\n…[truncated %d chars]", len(res.Text)-*maxLen)
	}

	if *copyBack || (*useClipboard && isPipeable()) {
		// Default when --clipboard is used without explicit output redirect:
		// put it back so the next paste is clean.
		if err := clipboard.WriteAll(out); err != nil {
			fatal("write clipboard: %v", err)
		}
	}

	fmt.Print(out)

	if *showStats || *copyBack {
		fmt.Fprintf(os.Stderr, "scrubbed %d value(s): %s\n",
			res.Replaces, strings.Join(redact.SortedCounts(res.Counts), " "))
	}
}

func isPipeable() bool {
	// If stdout is a terminal and --clipboard was used, the user almost
	// certainly wants the result back in the clipboard.
	fi, _ := os.Stdout.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func hasStdin() bool {
	fi, _ := os.Stdin.Stat()
	return (fi.Mode() & os.ModeCharDevice) == 0
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "prompt-parse: "+format+"\n", args...)
	os.Exit(1)
}
