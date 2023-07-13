package utils

import (
	"fmt"
	golog "log"
)

var (
	verbose bool
)

// Logs an error and exits the application.
func Error(message string, v ...any) {
	log(levelError, message, v...)
}

// Logs a warning.
func Warn(message string, v ...any) {
	log(levelWarn, message, v...)
}

// Logs an information.
func Info(message string, v ...any) {
	log(levelInfo, message, v...)
}

// Logs debug messages.
func Debug(message string, v ...any) {
	if verbose {
		log(levelDebug, message, v...)
	}
}

// Enables verbose output (debug).
func SetVerbose() {
	verbose = true
}

// Log a formatted string with the configured level.
func log(
	level logLevel,
	message string,
	v ...any,
) {
	logs[level](
		fmt.Sprintf("[%s]: %s", level, message),
		v...,
	)
}

// Defines our log level.
type logLevel string

// Supported log levels.
const (
	levelError logLevel = "ERROR"
	levelInfo  logLevel = "INFO"
	levelWarn  logLevel = "WARN"
	levelDebug logLevel = "DEBUG"
)

// Log level mappings to the real log function.
var logs = map[logLevel]func(string, ...any){
	levelError: golog.Fatalf,
	levelWarn:  golog.Printf,
	levelInfo:  golog.Printf,
	levelDebug: golog.Printf,
}
