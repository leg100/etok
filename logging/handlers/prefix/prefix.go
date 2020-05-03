// Package prefix implements a colored text handler suitable for command-line interfaces with a
// configurable prefix
package prefix

import (
	"io"
	"sync"

	"github.com/apex/log"
)

// Handler implementation.
type Handler struct {
	mu      sync.Mutex
	Writer  io.Writer
	Handler log.Handler
	Prefix  string
}

// New handler.
func New(h log.Handler, prefix string) *Handler {
	return &Handler{
		Handler: h,
		Prefix:  prefix,
	}
}

// HandleLog implements log.Handler.
func (h *Handler) HandleLog(e *log.Entry) error {
	e.Message = h.Prefix + e.Message

	return h.Handler.HandleLog(e)
}
