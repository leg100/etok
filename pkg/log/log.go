package log

import (
	"fmt"
	"io"
	"os"
)

// Puts into practice Dave Cheney's blog post
// https://dave.cheney.net/2015/11/05/lets-talk-about-logging

// Level of severity.
type Level int

// Log levels
const (
	InfoLevel Level = iota
	DebugLevel
)

// Current log level
var level Level

// Current output device
var out io.Writer = os.Stdout

func SetLevel(lvl Level) {
	level = lvl
}

func SetOut(w io.Writer) {
	out = w
}

func Debug(msg string) {
	if level == DebugLevel {
		fmt.Fprintln(out, msg)
	}
}

func Debugf(format string, a ...interface{}) {
	if level == DebugLevel {
		fmt.Fprintf(out, format, a...)
	}
}

func Info(msg string) {
	if level >= InfoLevel {
		fmt.Fprintln(out, msg)
	}
}

func Infof(format string, a ...interface{}) {
	if level >= InfoLevel {
		fmt.Fprintf(out, format, a...)
	}
}
