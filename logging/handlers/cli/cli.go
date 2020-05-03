// [leg100]: copy and pasted from apex pkg:
// * reduced padding from 3 to 1
// * changed debug color from white to magneta (so I can see it on my solarized-light terminal
// scheme!)
//
// Package cli implements a colored text handler suitable for command-line interfaces.
package cli

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/fatih/color"
	colorable "github.com/mattn/go-colorable"
)

// Default handler
var Default = New(os.Stdout, os.Stderr)

// start time.
var start = time.Now()

var bold = color.New(color.Bold)

// Colors mapping.
var Colors = [...]*color.Color{
	log.DebugLevel: color.New(color.FgMagenta),
	log.InfoLevel:  color.New(color.FgBlue),
	log.WarnLevel:  color.New(color.FgYellow),
	log.ErrorLevel: color.New(color.FgRed),
	log.FatalLevel: color.New(color.FgRed),
}

// Strings mapping.
var Strings = [...]string{
	log.DebugLevel: "•",
	log.InfoLevel:  "•",
	log.WarnLevel:  "•",
	log.ErrorLevel: "⨯",
	log.FatalLevel: "⨯",
}

// Handler implementation.
type Handler struct {
	mu      sync.Mutex
	Stdout  io.Writer
	Stderr  io.Writer
	Padding int
}

// New handler.
func New(stdout io.Writer, stderr io.Writer) *Handler {
	if fOut, ok := stdout.(*os.File); ok {
		if fErr, ok := stderr.(*os.File); ok {
			return &Handler{
				Stdout:  colorable.NewColorable(fOut),
				Stderr:  colorable.NewColorable(fErr),
				Padding: 1,
			}
		}
	}

	return &Handler{
		Stdout:  stdout,
		Stderr:  stderr,
		Padding: 1,
	}
}

func (h *Handler) Writer(level log.Level) io.Writer {
	if level > log.InfoLevel {
		return h.Stderr
	} else {
		return h.Stdout
	}
}

// HandleLog implements log.Handler.
func (h *Handler) HandleLog(e *log.Entry) error {
	color := Colors[e.Level]
	level := Strings[e.Level]
	names := e.Fields.Names()

	h.mu.Lock()
	defer h.mu.Unlock()

	color.Fprintf(h.Writer(e.Level), "%s %-25s", bold.Sprintf("%*s", h.Padding+1, level), e.Message)

	for _, name := range names {
		if name == "source" {
			continue
		}
		fmt.Fprintf(h.Writer(e.Level), " %s=%v", color.Sprint(name), e.Fields.Get(name))
	}

	fmt.Fprintln(h.Writer(e.Level))

	return nil
}
