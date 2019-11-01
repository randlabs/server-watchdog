package logger

import (
	"errors"

	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

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
	FileLoggerError(channel, format, a...)
	SlackLoggerError(channel, format, a...)
	EmailLoggerError(channel, format, a...)
	return
}

func LogWarn(channel string, format string, a ...interface{}) {
	console.WarnWithTitle(channel, format, a...)
	FileLoggerWarn(channel, format, a...)
	SlackLoggerWarn(channel, format, a...)
	EmailLoggerWarn(channel, format, a...)
	return
}

func LogInfo(channel string, format string, a ...interface{}) {
	console.InfoWithTitle(channel, format, a...)
	FileLoggerInfo(channel, format, a...)
	SlackLoggerInfo(channel, format, a...)
	EmailLoggerInfo(channel, format, a...)
	return
}
