package github

import (
	"bytes"
	"net/http"
	"text/template"
	"time"

	"github.com/urfave/negroni"
	"k8s.io/klog/v2"
)

const loggerDefaultFormat = "{{.Status}} | {{.Duration}} | {{.Hostname}} | {{.Method}} {{.Path}}"

// Logger is a middleware handler that logs the request as it goes in and the
// response as it goes out.
type Logger struct {
	template *template.Template
}

// NewLogger returns a new Logger instance
func NewLogger() *Logger {
	l := &Logger{}
	l.SetFormat(loggerDefaultFormat)
	return l
}

func (l *Logger) SetFormat(format string) {
	l.template = template.Must(template.New("negroni_parser").Parse(format))
}

func (l *Logger) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	start := time.Now()

	next(rw, r)

	res := rw.(negroni.ResponseWriter)
	log := negroni.LoggerEntry{
		Status:   res.Status(),
		Duration: time.Since(start),
		Hostname: r.Host,
		Method:   r.Method,
		Path:     r.URL.Path,
		Request:  r,
	}

	buff := &bytes.Buffer{}
	l.template.Execute(buff, log)
	klog.V(1).Info("Served request: " + buff.String())
}
