package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// Logger provides a minimal logging interface for injection and testing.
type Logger interface {
	Info(args ...any)
	Infof(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Debug(args ...any)
	Debugf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
}

type StdLogger struct {
	l *log.Logger
}

func New(out io.Writer, prefix string, flag int) *StdLogger {
	if out == nil {
		out = os.Stdout
	}
	return &StdLogger{l: log.New(out, prefix, flag)}
}

func Default() *StdLogger {
	return New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
}

var DefaultLogger Logger = Default()

const callerDepth = 4

func (l *StdLogger) log(msg string) {
	_ = l.l.Output(callerDepth, msg)
}

func formatArgs(args ...any) string {
	return strings.TrimRight(fmt.Sprintln(args...), "\n")
}

func (l *StdLogger) Info(args ...any) {
	l.log("[INFO] " + formatArgs(args...))
}

func (l *StdLogger) Infof(format string, args ...any) {
	l.log("[INFO] " + fmt.Sprintf(format, args...))
}

func (l *StdLogger) Error(args ...any) {
	l.log("[ERROR] " + formatArgs(args...))
}

func (l *StdLogger) Errorf(format string, args ...any) {
	l.log("[ERROR] " + fmt.Sprintf(format, args...))
}

func (l *StdLogger) Debug(args ...any) {
	l.log("[DEBUG] " + formatArgs(args...))
}

func (l *StdLogger) Debugf(format string, args ...any) {
	l.log("[DEBUG] " + fmt.Sprintf(format, args...))
}

func (l *StdLogger) Fatal(args ...any) {
	l.log("[FATAL] " + formatArgs(args...))
	os.Exit(1)
}

func (l *StdLogger) Fatalf(format string, args ...any) {
	l.log("[FATAL] " + fmt.Sprintf(format, args...))
	os.Exit(1)
}
