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
	classInfo int = 0
	classWarn = 1
	classError = 2
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

func Info(format string, a ...interface{}) {
	printCommon("", classInfo, getTimestamp(), fmt.Sprintf(format, a...))
	return
}

func Warn(format string, a ...interface{}) {
	printCommon("", classWarn, getTimestamp(), fmt.Sprintf(format, a...))
	return
}

func Error(format string, a ...interface{}) {
	printCommon("", classError, getTimestamp(), fmt.Sprintf(format, a...))
	return
}

func LogInfo(title string, timestamp string, msg string) {
	printCommon(title, classInfo, timestamp, msg)
	return
}

func LogWarn(title string, timestamp string, msg string) {
	printCommon(title, classWarn, timestamp, msg)
	return
}

func LogError(title string, timestamp string, msg string) {
	printCommon(title, classError, timestamp, msg)
	return
}

//------------------------------------------------------------------------------

func printCommon(title string, cls int, timestamp string, msg string) {
	if service.Interactive() || serviceLogger == nil {
		m.Lock()
		defer m.Unlock()

		if cls == classInfo {
			color.SetOutput(os.Stdout)
		} else {
			color.SetOutput(os.Stderr)
		}

		color.Printf("%v ", timestamp)

		switch cls {
		case classInfo:
			color.Info.Print("[INFO]")
		case classWarn:
			color.Warn.Print("[WARN]")
		case classError:
			color.Error.Print("[ERROR]")
		}

		if len(title) > 0 {
			color.Printf(" %v", title)
		}

		color.Printf(" - %v\n", msg)

		color.ResetOutput()
	} else {
		if len(title) > 0 {
			switch cls {
			case classInfo:
				(*serviceLogger).Infof("[INFO] %v - %v", title, msg)
			case classWarn:
				(*serviceLogger).Warningf("[WARN] %v - %v", title, msg)
			case classError:
				(*serviceLogger).Errorf("[ERROR] %v - %v", title, msg)
			}
		} else {
			switch cls {
			case classInfo:
				(*serviceLogger).Infof("[INFO] - %v", msg)
			case classWarn:
				(*serviceLogger).Warningf("[WARN] - %v", msg)
			case classError:
				(*serviceLogger).Errorf("[ERROR] - %v", msg)
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
