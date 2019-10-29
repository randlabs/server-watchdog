package console

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gookit/color"
)

//------------------------------------------------------------------------------

var m sync.Mutex

//------------------------------------------------------------------------------

func Info(format string, a ...interface{}) {
	InfoWithTitle("", format, a...)
	return
}

func Warn(format string, a ...interface{}) {
	WarnWithTitle("", format, a...)
	return
}

func Error(format string, a ...interface{}) {
	ErrorWithTitle("", format, a...)
	return
}

func Fatal(format string, a ...interface{}) {
	Error(format, a)
	os.Exit(1)
}

func InfoWithTitle(title string, format string, a ...interface{}) {
	printCommon(title, color.Info, "INFO", format, a...)
	return
}

func WarnWithTitle(title string, format string, a ...interface{}) {
	printCommon(title, color.Warn, "WARN", format, a...)
	return
}

func ErrorWithTitle(title string, format string, a ...interface{}) {
	printCommon(title, color.Error, "ERROR", format, a...)
	return
}

//------------------------------------------------------------------------------

func printCommon(title string, theme *color.Theme, label string, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	now := time.Now().UTC()

	m.Lock()
	defer m.Unlock()

	color.Print(now.Format("2006-01-02 15:04:05") + " ")
	theme.Print("[" + label + "]", )
	if len(title) > 0 {
		color.Print(" " + title)
	}
	color.Print(" - " + msg + "\n", )
	return
}
