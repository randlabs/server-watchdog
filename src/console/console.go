package console

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gookit/color"
	"github.com/kardianos/service"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

const(
	classError int = 0
	classWarn = 1
	classInfo = 2
	classDebug = 3
)

//------------------------------------------------------------------------------

var m sync.Mutex
var serviceLogger *service.Logger

//------------------------------------------------------------------------------

func SetupService(s service.Service) error {
	lg, err := s.Logger(nil)
	if err == nil {
		serviceLogger = &lg
	}
	return err
}

func Error(format string, a ...interface{}) {
	printCommon("", classError, getTimestamp(), fmt.Sprintf(format, a...))
	return
}

func Warn(format string, a ...interface{}) {
	printCommon("", classWarn, getTimestamp(), fmt.Sprintf(format, a...))
	return
}

func Info(format string, a ...interface{}) {
	printCommon("", classInfo, getTimestamp(), fmt.Sprintf(format, a...))
	return
}

func Debug(format string, a ...interface{}) {
	printCommon("", classDebug, getTimestamp(), fmt.Sprintf(format, a...))
	return
}

func LogError(title string, timestamp string, msg string) {
	printCommon(title, classError, timestamp, msg)
	return
}

func LogWarn(title string, timestamp string, msg string) {
	printCommon(title, classWarn, timestamp, msg)
	return
}

func LogInfo(title string, timestamp string, msg string) {
	printCommon(title, classInfo, timestamp, msg)
	return
}

func LogDebug(title string, timestamp string, msg string) {
	printCommon(title, classDebug, timestamp, msg)
	return
}

//------------------------------------------------------------------------------

func printCommon(title string, cls int, timestamp string, msg string) {
	if service.Interactive() || serviceLogger == nil {
		m.Lock()
		defer m.Unlock()

		if cls == classInfo || cls == classDebug {
			color.SetOutput(os.Stdout)
		} else {
			color.SetOutput(os.Stderr)
		}

		color.Printf("%v ", timestamp)

		switch cls {
		case classError:
			color.Error.Print("[ERROR]")
		case classWarn:
			color.Warn.Print("[WARN]")
		case classInfo:
			color.Info.Print("[INFO]")
		case classDebug:
			color.Debug.Print("[DEBUG]")
		}

		if len(title) > 0 {
			color.Printf(" %v", title)
		}

		color.Printf(" - %v\n", msg)

		color.ResetOutput()
	} else {
		if len(title) > 0 {
			switch cls {
			case classError:
				(*serviceLogger).Errorf("[ERROR] %v - %v", title, msg)
			case classWarn:
				(*serviceLogger).Warningf("[WARN] %v - %v", title, msg)
			case classInfo:
				(*serviceLogger).Infof("[INFO] %v - %v", title, msg)
			case classDebug:
				(*serviceLogger).Infof("[DEBUG] %v - %v", title, msg)
			}
		} else {
			switch cls {
			case classError:
				(*serviceLogger).Errorf("[ERROR] - %v", msg)
			case classWarn:
				(*serviceLogger).Warningf("[WARN] - %v", msg)
			case classInfo:
				(*serviceLogger).Infof("[INFO] - %v", msg)
			case classDebug:
				(*serviceLogger).Infof("[DEBUG] - %v", msg)
			}
		}
	}
	return
}

func getTimestamp() string {
	now := time.Now()
	if !settings.Config.Log.UseLocalTime {
		now = now.UTC()
	}
	return now.Format("2006-01-02 15:04:05")
}
