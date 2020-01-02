package console

import (
	"fmt"
	"sync"
	"time"

	"github.com/gookit/color"
	"github.com/kardianos/service"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

var m sync.Mutex

//------------------------------------------------------------------------------

func Print(format string, a ...interface{}) {
	if service.Interactive() {
		color.Print(fmt.Sprintf(format, a...))
	}
}

func Println(format string, a ...interface{}) {
	if service.Interactive() {
		color.Println(fmt.Sprintf(format, a...))
	}
}

func Info(format string, a ...interface{}) {
	if service.Interactive() {
		printCommon("", color.Info, "INFO", getTimestamp(), fmt.Sprintf(format, a...))
	}
}

func Warn(format string, a ...interface{}) {
	if service.Interactive() {
		printCommon("", color.Warn, "WARN", getTimestamp(), fmt.Sprintf(format, a...))
	}
}

func Error(format string, a ...interface{}) {
	if service.Interactive() {
		printCommon("", color.Error, "ERROR", getTimestamp(), fmt.Sprintf(format, a...))
	}
}

func PrintlnSuccess() {
	if service.Interactive() {
		color.LightGreen.Println("OK")
	}
}

func PrintlntError(format string, a ...interface{}) {
	if service.Interactive() {
		color.Error.Println(fmt.Sprintf(format, a...))
	}
}

func LogInfo(title string, timestamp string, msg string) {
	if service.Interactive() {
		printCommon(title, color.Info, "INFO", timestamp, msg)
	}
}

func LogWarn(title string, timestamp string, msg string) {
	printCommon(title, color.Warn, "WARN", timestamp, msg)
	return
}

func LogError(title string, timestamp string, msg string) {
	if service.Interactive() {
		printCommon(title, color.Error, "ERROR", timestamp, msg)
	}
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
