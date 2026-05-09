package logger

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

const (
	colorRed     = "\x1b[31m"
	colorGreen   = "\x1b[32m"
	colorYellow  = "\x1b[33m"
	colorBlue    = "\x1b[34m"
	colorMagenta = "\x1b[35m"
	colorReset   = "\x1b[0m"
)

type Logger struct {
	useColors bool
	verbose   bool
}

func NewLogger(verbose bool) *Logger {
	return &Logger{
		useColors: checkColorSupport(verbose),
		verbose:   verbose,
	}
}

func checkColorSupport(verbose bool) bool {
	if os.Getenv("NO_COLOR") != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "Colors disabled due to NO_COLOR environment variable\n")
		}
		return false
	}

	termEnv := os.Getenv("TERM")
	if termEnv == "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "No TERM environment variable found\n")
		}
		return false
	}

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		if verbose {
			fmt.Fprintf(os.Stderr, "Output is not going to a terminal\n")
		}
		return false
	}

	colorTerms := []string{"xterm", "xterm-256color", "screen", "screen-256color", "tmux", "tmux-256color", "linux"}
	for _, colorTerm := range colorTerms {
		if strings.HasPrefix(termEnv, colorTerm) {
			if verbose {
				fmt.Fprintf(os.Stderr, "Color support detected for TERM=%s\n", termEnv)
			}
			return true
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "TERM=%s doesn't appear to support colors\n", termEnv)
	}
	return false
}

func (l *Logger) Colorize(color, text string) string {
	if l.useColors {
		result := color + text + colorReset
		if l.verbose {
			fmt.Fprintf(os.Stderr, "Colorizing: input=%q, with_color=%q\n", text, result)
		}
		return result
	}
	if l.verbose {
		fmt.Fprintf(os.Stderr, "Colors disabled for text: %q\n", text)
	}
	return text
}

func (l *Logger) Info(msg string) {
	prefix := l.Colorize(colorGreen, "[INFO]")
	fmt.Printf("%s %s\n", prefix, msg)
}

func (l *Logger) Warning(msg string) {
	prefix := l.Colorize(colorYellow, "[WARNING]")
	fmt.Printf("%s %s\n", prefix, msg)
}

func (l *Logger) Error(msg string) {
	prefix := l.Colorize(colorRed, "[ERROR]")
	fmt.Printf("%s %s\n", prefix, msg)
}

func (l *Logger) Debug(msg string) {
	if l.verbose {
		prefix := l.Colorize(colorBlue, "[DEBUG]")
		fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
	}
}
