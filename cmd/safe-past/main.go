// Command safe-past: clipboard-first PII scrubber.
//
// Typical flow (no app to open):
//  1. Copy logs from prod.
//  2. Run global hotkey -> `safe-past --clipboard --copy`
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
	"github.com/idarciooliveira/safe-past/internal/redact"
)

var version = "dev"

func main() {
	var (
		useClipboard = flag.Bool("clipboard", false, "read input from clipboard instead of stdin/file args")
		copyBack     = flag.Bool("copy", false, "write cleaned text back to clipboard")
		showStats    = flag.Bool("stats", false, "print redaction stats to stderr")
		pattern      = flag.String("pattern", "", "extra custom regex (comma-separated)")
		configFile   = flag.String("config", "", "file with one extra regex per line (# = comment)")
		maxLen       = flag.Int("max", 0, "truncate output to N chars (0 = no limit)")
		listRules    = flag.Bool("list-rules", false, "list built-in detection rules and exit")
		showVersion  = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("safe-past " + version)
		return
	}
	if *listRules {
		for _, r := range redact.Rules() {
			fmt.Printf("%-16s -> [%s_N]\n", r.Name, r.Placeholder)
		}
		return
	}

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
		fmt.Fprintln(os.Stderr, "\nExample: safe-past --clipboard --copy --stats")
		os.Exit(2)
	}

	var extras []string
	if *pattern != "" {
		extras = strings.Split(*pattern, ",")
	}
	if *configFile != "" {
		b, err := os.ReadFile(*configFile)
		if err != nil {
			fatal("read config %s: %v", *configFile, err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			extras = append(extras, line)
		}
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
	fmt.Fprintf(os.Stderr, "safe-past: "+format+"\n", args...)
	os.Exit(1)
}
