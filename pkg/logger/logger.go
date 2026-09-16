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
	colorCyan    = "\x1b[36m"
	colorGray    = "\x1b[90m"
	colorBrightGreen = "\x1b[92m"
	colorReset   = "\x1b[0m"
)

const (
	LevelInfo    = 0
	LevelVerbose = 1
	LevelDebug   = 2
	LevelTrace   = 3
)

type Logger struct {
	useColors bool
	verbosity int
}

func NewLogger(verbosity int) *Logger {
	return &Logger{
		useColors: checkColorSupport(verbosity),
		verbosity: verbosity,
	}
}

func checkColorSupport(verbosity int) bool {
	if os.Getenv("NO_COLOR") != "" {
		if verbosity >= LevelDebug {
			fmt.Fprintf(os.Stderr, "Colors disabled due to NO_COLOR environment variable\n")
		}
		return false
	}

	termEnv := os.Getenv("TERM")
	if termEnv == "" {
		if verbosity >= LevelDebug {
			fmt.Fprintf(os.Stderr, "No TERM environment variable found\n")
		}
		return false
	}

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		if verbosity >= LevelDebug {
			fmt.Fprintf(os.Stderr, "Output is not going to a terminal\n")
		}
		return false
	}

	colorTerms := []string{"xterm", "xterm-256color", "screen", "screen-256color", "tmux", "tmux-256color", "linux"}
	for _, colorTerm := range colorTerms {
		if strings.HasPrefix(termEnv, colorTerm) {
			if verbosity >= LevelTrace {
				fmt.Fprintf(os.Stderr, "Color support detected for TERM=%s\n", termEnv)
			}
			return true
		}
	}

	if verbosity >= LevelDebug {
		fmt.Fprintf(os.Stderr, "TERM=%s doesn't appear to support colors\n", termEnv)
	}
	return false
}

func (l *Logger) Colorize(color, text string) string {
	if l.useColors {
		result := color + text + colorReset
		if l.verbosity >= LevelTrace {
			fmt.Fprintf(os.Stderr, "Colorizing: input=%q, with_color=%q\n", text, result)
		}
		return result
	}
	if l.verbosity >= LevelTrace {
		fmt.Fprintf(os.Stderr, "Colors disabled for text: %q\n", text)
	}
	return text
}

func (l *Logger) Info(msg string) {
	prefix := l.Colorize(colorCyan, "[INFO]")
	fmt.Printf("%s %s\n", prefix, msg)
}

func (l *Logger) Success(msg string) {
	prefix := l.Colorize(colorBrightGreen, "[SUCCESS]")
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

func (l *Logger) Verbose(msg string) {
	if l.verbosity >= LevelVerbose {
		prefix := l.Colorize(colorMagenta, "[VERBOSE]")
		fmt.Printf("%s %s\n", prefix, msg)
	}
}

func (l *Logger) Debug(msg string) {
	if l.verbosity >= LevelDebug {
		prefix := l.Colorize(colorBlue, "[DEBUG]")
		fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
	}
}

func (l *Logger) Trace(msg string) {
	if l.verbosity >= LevelTrace {
		prefix := l.Colorize(colorGray, "[TRACE]")
		fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
	}
}
