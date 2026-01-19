package logger

import (
	"io"
	"log"
	"os"
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

type MyLogger struct {
	l *log.Logger
}

func New(out io.Writer, prefix string, flag int) *MyLogger {
	if out == nil {
		out = os.Stdout
	}
	return &MyLogger{l: log.New(out, prefix, flag)}
}

func Default() *MyLogger {
	return New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
}

var DefaultLogger Logger = Default()

func (l *MyLogger) Info(args ...any) {
	l.l.Println(append([]any{"[INFO]"}, args...)...)
}

func (l *MyLogger) Infof(format string, args ...any) {
	l.l.Printf("[INFO] "+format+"\n", args...)
}

func (l *MyLogger) Error(args ...any) {
	l.l.Println(append([]any{"[ERROR]"}, args...)...)
}

func (l *MyLogger) Errorf(format string, args ...any) {
	l.l.Printf("[ERROR] "+format+"\n", args...)
}

func (l *MyLogger) Debug(args ...any) {
	l.l.Println(append([]any{"[DEBUG]"}, args...)...)
}

func (l *MyLogger) Debugf(format string, args ...any) {
	l.l.Printf("[DEBUG] "+format+"\n", args...)
}

func (l *MyLogger) Fatal(args ...any) {
	l.l.Fatal(append([]any{"[FATAL]"}, args...)...)
}

func (l *MyLogger) Fatalf(format string, args ...any) {
	l.l.Fatalf("[FATAL] "+format, args...)
}
