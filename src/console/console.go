package console

import (
	"fmt"
	"github.com/randlabs/server-watchdog/settings"
	"os"
	"sync"
	"time"

	"github.com/gookit/color"
)

//------------------------------------------------------------------------------

var m sync.Mutex

//------------------------------------------------------------------------------

func Info(format string, a ...interface{}) {
	printCommon("", color.Info, "INFO", getTimestamp(), fmt.Sprintf(format, a...))
	return
}

func Warn(format string, a ...interface{}) {
	printCommon("", color.Warn, "WARN", getTimestamp(), fmt.Sprintf(format, a...))
	return
}

func Error(format string, a ...interface{}) {
	printCommon("", color.Error, "ERROR", getTimestamp(), fmt.Sprintf(format, a...))
	return
}

func Fatal(format string, a ...interface{}) {
	Error(format, a)
	os.Exit(1)
}

func LogInfo(title string, timestamp string, msg string) {
	printCommon(title, color.Info, "INFO", timestamp, msg)
	return
}

func LogWarn(title string, timestamp string, msg string) {
	printCommon(title, color.Warn, "WARN", timestamp, msg)
	return
}

func LogError(title string, timestamp string, msg string) {
	printCommon(title, color.Error, "ERROR", timestamp, msg)
	return
}

//------------------------------------------------------------------------------

func printCommon(title string, theme *color.Theme, label string, timestamp string, msg string) {
	m.Lock()
	defer m.Unlock()

	color.Print(timestamp + " ")
	theme.Print("[" + label + "]", )
	if len(title) > 0 {
		color.Print(" " + title)
	}
	color.Print(" - " + msg + "\n", )
	return
}

func getTimestamp() string {
	now := time.Now()
	if !settings.Config.Log.UseLocalTime {
		now = now.UTC()
	}
	return now.Format("2006-01-02 15:04:05")
}
