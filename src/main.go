package main

import (
	"os"
	"sync"

	"github.com/gookit/color"
	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/modules"
	"github.com/randlabs/server-watchdog/modules/backend"
	"github.com/randlabs/server-watchdog/modules/logger"
	"github.com/randlabs/server-watchdog/settings"
	"github.com/randlabs/server-watchdog/utils/process"
)

//------------------------------------------------------------------------------

var wg sync.WaitGroup

//------------------------------------------------------------------------------

func main() {
	var err error

	color.Print("Initializing... ")

	err = settings.Load()
	if err != nil {
		color.Println("")
		console.Fatal("%v", err.Error())
	}

	err = logger.FileLoggerStart()
	if err != nil {
		color.Println("")
		console.Error("Unable to create file logger [%v]", err)
		goto Done
	}

	err = logger.SlackLoggerStart()
	if err != nil {
		color.Println("")
		console.Error("Unable to create Slack logger [%v]", err)
		goto Done
	}

	err = modules.ProcessWatcherStart()
	if err != nil {
		color.Println("")
		console.Error("Unable to create process monitor [%v]", err)
		goto Done
	}

	err = modules.FreeDiskSpaceCheckerStart()
	if err != nil {
		color.Println("")
		console.Error("Unable to create process monitor [%v]", err)
		goto Done
	}

	err = modules.WebCheckerStart()
	if err != nil {
		color.Println("")
		console.Error("Unable to create process monitor [%v]", err)
		goto Done
	}

	err = backend.BackendStart()
	if err != nil {
		color.Println("")
		console.Error("Unable to create server [%v]", err)
		goto Done
	}

	color.LightGreen.Println("OK")

	logger.FileLoggerRun(wg)
	logger.SlackLoggerRun(wg)
	modules.ProcessWatcherRun(wg)
	modules.WebCheckerRun(wg)
	modules.FreeDiskSpaceCheckerRun(wg)
	backend.BackendRun(wg)

	<-process.GetShutdownSignal()

	err = nil

Done:
	if err == nil {
		color.Print("Shutting down... ")
	}

	backend.BackendStop()
	modules.FreeDiskSpaceCheckerStop()
	modules.WebCheckerStop()
	modules.ProcessWatcherStop()
	logger.SlackLoggerStop()
	logger.FileLoggerStop()

	wg.Wait()

	if err == nil {
		color.LightGreen.Println("OK")
	}

	if err != nil {
		os.Exit(1)
	}
	return
}

