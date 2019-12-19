package logger

import (
	"errors"
	"sync"

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

	default:
		return errors.New("Invalid severity")
	}

	return nil
}

func LogError(channel string, format string, a ...interface{}) {
	console.ErrorWithTitle(channel, format, a...)
	file.Error(channel, format, a...)
	slack.Error(channel, format, a...)
	email.Error(channel, format, a...)
	return
}

func LogWarn(channel string, format string, a ...interface{}) {
	console.WarnWithTitle(channel, format, a...)
	file.Warn(channel, format, a...)
	slack.Warn(channel, format, a...)
	email.Warn(channel, format, a...)
	return
}

func LogInfo(channel string, format string, a ...interface{}) {
	console.InfoWithTitle(channel, format, a...)
	file.Info(channel, format, a...)
	slack.Info(channel, format, a...)
	email.Info(channel, format, a...)
	return
}
