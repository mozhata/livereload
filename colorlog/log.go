/*
usage:
logger = NewLogger("prefix")
logger.Trace("this a test of Trace")
logger.Info("this is a test of Info")
logger.Success("this is a test of Success")
logger.Warning("this is a test of Warning")
logger.Error("this is a test of Error")
*/

package colorlog

import (
	"fmt"
	"log"
	"os"
)

const (
	Red = uint8(iota + 91)
	Green
	Yellow
	Blue
	Magenta

	INFO = "INFO"
	TRAC = "TRAC"
	ERRO = "ERRO"
	WARN = "WARN"
	SUCC = "SUCC"
)

// Logger wapper log.Logger
type Logger struct {
	*log.Logger
}

// NewLogger make log.New easy
func NewLogger(prefix string) *Logger {
	inner := log.New(os.Stderr, prefix, log.Lmicroseconds)
	return &Logger{inner}
}

// Trace output formatted string to stderr in blue color
// will append '\n' at the last
func (l *Logger) Trace(format string, a ...interface{}) {
	prefix := blue(TRAC)
	l.Println(prefix, fmt.Sprintf(format, a...))
}

// Info output formatted string to stderr in blue color
// will append '\n' at the last
func (l *Logger) Info(format string, a ...interface{}) {
	prefix := blue(INFO)
	l.Println(prefix, fmt.Sprintf(format, a...))
}

// Success output formatted string to stderr in green color
// will append '\n' at the last
func (l *Logger) Success(format string, a ...interface{}) {
	prefix := green(SUCC)
	l.Println(prefix, fmt.Sprintf(format, a...))
}

// Warning output formatted string to stderr in Magenta color
// will append '\n' at the last
func (l *Logger) Warning(format string, a ...interface{}) {
	prefix := magenta(WARN)
	l.Println(prefix, fmt.Sprintf(format, a...))
}

// Error output formatted string to stderr in red color
// will append '\n' at the last
func (l *Logger) Error(format string, a ...interface{}) {
	prefix := red(ERRO)
	l.Println(prefix, fmt.Sprintf(format, a...))
}

func blue(s string) string {
	return fmt.Sprintf("[\033[%dm%s\033[0m]", Blue, s)
}
func green(s string) string {
	return fmt.Sprintf("[\033[%dm%s\033[0m]", Green, s)
}
func magenta(s string) string {
	return fmt.Sprintf("[\033[%dm%s\033[0m]", Magenta, s)
}
func red(s string) string {
	return fmt.Sprintf("[\033[%dm%s\033[0m]", Red, s)
}
