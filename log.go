/*
Package log is a standard logging system for IMQS Go applications.

This is a very thin wrapper around lumberjack. What this package provides
is a consistent log format, with predefined severity levels.

# Usage

Create a new logger using log.New(filename, runtime.GOOS != "windows").
You can write to it using the various logging methods.
'filename' may also be log.Stdout or log.Stderr, in which case we do the obvious thing.

If you need to forward other log messages to this system, then Forwarder might have
sufficient functionality to achieve that.
*/
package log

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Level int

const (
	Trace Level = iota
	Debug
	Info
	Warn
	Error
)

const (
	Stdout  = "stdout"
	Stderr  = "stderr"
	Testing = ".testing."
)

// ISO 8601, with 6 digits of time precision
const timeFormat = "2006-01-02T15:04:05.000000Z0700"

// This variable is initialized the first time New is called and then returned
// on every subsequent call. It can be used throughout an application by
// just importing this package into any package
var Log *Logger

func levelToName(level Level) string {
	switch level {
	case Trace:
		return "Trace"
	case Debug:
		return "Debug"
	case Info:
		return "Info"
	case Warn:
		return "Warning"
	case Error:
		return "Error"
	}
	panic("Unknown log level")
}

// A logger object. Use New() to construct one.
type Logger struct {
	Level      Level // Log messages with a level lower than this are discarded. Default level is Info
	testing    *testing.T
	filename   string
	log        io.Writer
	shownError bool
	inDocker   bool
}

// New creates a new logger. If logToStdout is true all logs will be written to
// both the specified file and stdout. Filename may also be one of the special
// names log.Stdout and log.Stderr. If log.Stdout is specified and logToStdout
// is also set to true then the logs will only be written to stdout.
func New(filename string, logToStdout bool) *Logger {
	if Log != nil {
		return Log
	}
	l := &Logger{
		Level:    Info,
		filename: filename,
	}
	if _, err := os.Stat("/.dockerenv"); !os.IsNotExist(err) {
		l.inDocker = true
	}

	if filename == Stdout {
		l.log = io.Writer(os.Stdout)
	} else if filename == Stderr {
		l.log = io.Writer(os.Stderr)
	} else {
		lj := &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    30,
			MaxBackups: 3,
		}
		l.log = io.Writer(lj)
	}

	// We always log to stdout for docker
	if (l.inDocker || logToStdout) && filename != Stdout {
		l.log = io.MultiWriter(os.Stdout, l.log)
	}

	Log = l

	return l
}

// NewTesting creates a logger object that emits logs to the given Testing context
func NewTesting(t *testing.T) *Logger {
	return &Logger{
		Level:   Info,
		testing: t,
	}
}

// Close attempts to close the connection
func (l *Logger) Close() error {
	if wc, ok := l.log.(io.WriteCloser); ok {
		return wc.Close()
	} else if s, ok := l.log.(*os.File); ok {
		// Close connection to stdout/stderr
		return s.Close()
	}
	return nil
}

// Parse a level string such as "info" or "warn". Only the first character of the string is considered.
func ParseLevel(lev string) (Level, error) {
	if len(lev) != 0 {
		char0 := lev[0:1]
		if char0 == "T" || char0 == "t" {
			return Trace, nil
		}
		if char0 == "D" || char0 == "d" {
			return Debug, nil
		}
		if char0 == "I" || char0 == "i" {
			return Info, nil
		}
		if char0 == "W" || char0 == "w" {
			return Warn, nil
		}
		if char0 == "E" || char0 == "e" {
			return Error, nil
		}
	}
	return Info, fmt.Errorf("Invalid log level '%v'", lev)
}

func (l *Logger) Tracef(format string, params ...interface{}) {
	l.Logf(Trace, format, params...)
}

func (l *Logger) Debugf(format string, params ...interface{}) {
	l.Logf(Debug, format, params...)
}

func (l *Logger) Infof(format string, params ...interface{}) {
	l.Logf(Info, format, params...)
}

func (l *Logger) Warnf(format string, params ...interface{}) {
	l.Logf(Warn, format, params...)
}

func (l *Logger) Errorf(format string, params ...interface{}) {
	l.Logf(Error, format, params...)
}

func (l *Logger) Trace(msg string) {
	l.Log(Trace, msg)
}

func (l *Logger) Debug(msg string) {
	l.Log(Debug, msg)
}

func (l *Logger) Info(msg string) {
	l.Log(Info, msg)
}

func (l *Logger) Warn(msg string) {
	l.Log(Warn, msg)
}

func (l *Logger) Error(msg string) {
	l.Log(Error, msg)
}

func (l *Logger) Logf(level Level, format string, params ...interface{}) {
	if level >= l.Level {
		l.Log(level, fmt.Sprintf(format, params...))
	}
}

func (l *Logger) Log(level Level, msg string) {
	if level >= l.Level {
		suffix := ""
		if len(msg) == 0 || msg[len(msg)-1] != '\n' {
			suffix = "\n"
		}
		if l.testing != nil {
			l.testing.Logf("[%v] %v", levelToName(level)[0:1], msg)
		} else {
			s := fmt.Sprintf("%v [%v] %v%v", time.Now().Format(timeFormat), levelToName(level)[0:1], msg, suffix)
			l.Write([]byte(s))
		}
	}
}

func (l *Logger) Write(p []byte) (n int, err error) {
	n, err = l.log.Write(p)
	if err != nil && !l.shownError {
		l.shownError = true
		fmt.Printf("Unable to write to log file %v: %v. This error will not be shown again.\n", l.filename, err)
	}
	return
}

// Forwards log messages to an existing Logger, while performing some sanitizing which
// ensures that all log messages share the same format
type Forwarder struct {
	StripPrefixLen int     // Number of bytes of prefix to strip (typically the timestamp from the incoming log message)
	Level          Level   // The log level assigned to all messages from this source
	Target         *Logger // The destination
}

// Create a new log forwarder
func NewForwarder(stripPrefixLen int, level Level, target *Logger) *Forwarder {
	return &Forwarder{
		StripPrefixLen: stripPrefixLen,
		Level:          level,
		Target:         target,
	}
}

func (f *Forwarder) Write(p []byte) (n int, err error) {
	if len(p) > f.StripPrefixLen {
		f.Target.Log(f.Level, string(p[f.StripPrefixLen:]))
	}
	return len(p), nil
}
