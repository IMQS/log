/*
Package log is a standard logging system for IMQS Go applications.

This is a very thin wrapper around lumberjack. What this package provides
is a consistent log format, with predefined severity levels.

Usage

Create a new logger using log.New(filename).
You can write to it using the various logging methods.
'filename' may also be log.Stdout or log.Stderr, in which case we do the obvious thing.

log.Logger implements io.Writer, so you can chain other logging systems behind this.
*/
package log

import (
	"fmt"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"time"
)

type Level int

const (
	Trace Level = iota
	Debug
	Info
	Warn
	Error
)

const Stdout = "stdout"
const Stderr = "stderr"

// ISO 8601, with 6 digits of time precision
const timeFormat = "2006-01-02T15:04:05.000000Z0700"

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
	lj         lumberjack.Logger
	shownError bool
}

// Create a new logger. Filename may also be one of the special names log.Stdout and log.Stderr
func New(filename string) *Logger {
	l := &Logger{
		Level: Info,
	}
	l.lj.Filename = filename
	l.lj.MaxSize = 30
	l.lj.MaxBackups = 3
	return l
}

func (l *Logger) Close() error {
	return l.lj.Close()
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
		s := fmt.Sprintf("%v [%v] %v%v", time.Now().Format(timeFormat), levelToName(level)[0:1], msg, suffix)
		l.Write([]byte(s))
	}
}

func (l *Logger) Write(p []byte) (n int, err error) {
	if l.lj.Filename == Stdout {
		n, err = os.Stdout.Write(p)
	} else if l.lj.Filename == Stderr {
		n, err = os.Stderr.Write(p)
	} else {
		n, err = l.lj.Write(p)
		if err != nil && !l.shownError {
			l.shownError = true
			fmt.Printf("Unable to write to log file %v: %v. This error will not be shown again.\n", l.lj.Filename, err)
		}
	}
	return
}
