package logger

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/modules/logger/email"
	"github.com/randlabs/server-watchdog/modules/logger/file"
	"github.com/randlabs/server-watchdog/modules/logger/slack"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

func Start() error {
	err := file.Start()
	if err == nil {
		err = email.Start()
	}
	if err == nil {
		err = slack.Start()
	}
	if err != nil {
		Stop()
	}
	return err
}

func Stop() {
	email.Stop()
	slack.Stop()
	file.Stop()
	return
}

func Run(wg sync.WaitGroup) {
	email.Run(wg)
	slack.Run(wg)
	file.Run(wg)
	return
}

func Log(severity string, channel string, format string, a ...interface{}) error {
	switch settings.ValidateSeverity(severity) {
	case "error":
		LogError(channel, format, a...)

	case "warn":
		LogWarn(channel, format, a...)

	case "info":
		LogInfo(channel, format, a...)

	case "debug":
		LogDebug(channel, format, a...)

	default:
		return errors.New("Invalid severity")
	}

	return nil
}

func LogError(channel string, format string, a ...interface{}) {
	timestamp := getTimestamp()
	msg := fmt.Sprintf(format, a...)

	console.LogError(channel, timestamp, msg)
	file.Error(channel, timestamp, msg)
	slack.Error(channel, timestamp, msg)
	email.Error(channel, timestamp, msg)
	return
}

func LogWarn(channel string, format string, a ...interface{}) {
	timestamp := getTimestamp()
	msg := fmt.Sprintf(format, a...)

	console.LogWarn(channel, timestamp, msg)
	file.Warn(channel, timestamp, msg)
	slack.Warn(channel, timestamp, msg)
	email.Warn(channel, timestamp, msg)
	return
}

func LogInfo(channel string, format string, a ...interface{}) {
	timestamp := getTimestamp()
	msg := fmt.Sprintf(format, a...)

	console.LogInfo(channel, timestamp, msg)
	file.Info(channel, timestamp, msg)
	slack.Info(channel, timestamp, msg)
	email.Info(channel, timestamp, msg)
	return
}

func LogDebug(channel string, format string, a ...interface{}) {
	timestamp := getTimestamp()
	msg := fmt.Sprintf(format, a...)

	console.LogDebug(channel, timestamp, msg)
	file.Debug(channel, timestamp, msg)
	slack.Debug(channel, timestamp, msg)
	email.Debug(channel, timestamp, msg)
	return
}

func getTimestamp() string {
	now := time.Now()
	if !settings.Config.Log.UseLocalTime {
		now = now.UTC()
	}
	return now.Format("2006-01-02 15:04:05")
}
