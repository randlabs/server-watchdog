package main

import (
	"os"
	"sync"

	"github.com/gookit/color"
	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/modules/backend"
	"github.com/randlabs/server-watchdog/modules/freediskspacechecker"
	"github.com/randlabs/server-watchdog/modules/logger"
	"github.com/randlabs/server-watchdog/modules/processwatcher"
	"github.com/randlabs/server-watchdog/modules/webchecker"
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

	err = logger.Start() // Must be initialize first
	if err != nil {
		color.Println("")
		console.Error("Unable to create loggers [%v]", err)
		goto Done
	}

	err = processwatcher.Start()
	if err != nil {
		color.Println("")
		console.Error("Unable to create process monitor [%v]", err)
		goto Done
	}

	err = freediskspacechecker.Start()
	if err != nil {
		color.Println("")
		console.Error("Unable to create process monitor [%v]", err)
		goto Done
	}

	err = webchecker.Start()
	if err != nil {
		color.Println("")
		console.Error("Unable to create process monitor [%v]", err)
		goto Done
	}

	err = backend.Start()
	if err != nil {
		color.Println("")
		console.Error("Unable to create server [%v]", err)
		goto Done
	}

	color.LightGreen.Println("OK")

	console.Info("Running server at port %v", settings.Config.Server.Port)

	logger.Run(wg)
	processwatcher.Run(wg)
	webchecker.Run(wg)
	freediskspacechecker.Run(wg)
	backend.Run(wg)

	<-process.GetShutdownSignal()

	err = nil

Done:
	if err == nil {
		color.Print("Shutting down... ")
	}

	backend.Stop()
	freediskspacechecker.Stop()
	webchecker.Stop()
	processwatcher.Stop()
	logger.Stop()

	wg.Wait()

	if err == nil {
		color.LightGreen.Println("OK")
	}

	if err != nil {
		os.Exit(1)
	}
	return
}

