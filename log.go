package main

import (
	"io"
	"log"
	"os"

	"github.com/canonical/go-dqlite/client"
)

type LogFunc = client.LogFunc
type LogLevel = client.LogLevel

const (
	LogDebug = client.LogDebug
	LogInfo  = client.LogInfo
	LogWarn  = client.LogWarn
	LogError = client.LogError
)

// NewLogFunc returns a LogFunc.
func NewLogFunc(level LogLevel, prefix string, w io.Writer) LogFunc {
	if w == nil {
		w = os.Stdout
	}
	logger := log.New(w, prefix, log.LstdFlags|log.Lmicroseconds)
	return func(l LogLevel, format string, args ...interface{}) {
		if l >= level {
			// prepend the log level to the message
			args = append([]interface{}{l.String()}, args...)
			format = "[%s] " + format
			logger.Printf(format, args...)
		}
	}
}

// PanicLogFunc returns a LogFunc that panics if the log message is "panic".
func PanicLogFunc(level LogLevel, prefix string, w io.Writer) LogFunc {
	if w == nil {
		w = os.Stdout
	}
	logger := log.New(w, prefix, log.LstdFlags|log.Lmicroseconds)
	return func(l LogLevel, format string, args ...interface{}) {
		if format == "panic" {
			log.Panic("panic has been induced")
		}
		if l >= level {
			// prepend the log level to the message
			args = append([]interface{}{l.String()}, args...)
			format = "[%s] " + format
			logger.Printf(format, args...)
		}
	}
}
