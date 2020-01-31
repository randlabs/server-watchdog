package main

import (
	"github.com/kardianos/service"
	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/modules/backend"
	"github.com/randlabs/server-watchdog/modules/freediskspacechecker"
	"github.com/randlabs/server-watchdog/modules/logger"
	"github.com/randlabs/server-watchdog/modules/processwatcher"
	"github.com/randlabs/server-watchdog/modules/tcpports"
	"github.com/randlabs/server-watchdog/modules/webchecker"
	"github.com/randlabs/server-watchdog/settings"
	"github.com/randlabs/server-watchdog/utils/process"
	"runtime"
	"sync"
)

//------------------------------------------------------------------------------

type program struct {
	initiateShutdown chan struct{}
	wg sync.WaitGroup
}

//------------------------------------------------------------------------------

func (p *program) Start(s service.Service) error {
	p.initiateShutdown = make(chan struct{}, 1)

	err := settings.Load()
	if err != nil {
		console.Error("%v", err.Error())
		goto Done
	}

	err = logger.Start() // Must be initialize first
	if err != nil {
		console.Error("Unable to create loggers [%v]", err.Error())
		goto Done
	}

	err = processwatcher.Start()
	if err != nil {
		console.Error("Unable to create process monitor [%v]", err.Error())
		goto Done
	}

	err = freediskspacechecker.Start()
	if err != nil {
		console.Error("Unable to create free disk space monitor [%v]", err.Error())
		goto Done
	}

	err = webchecker.Start()
	if err != nil {
		console.Error("Unable to create web checker monitor [%v]", err.Error())
		goto Done
	}

	err = tcpports.Start()
	if err != nil {
		console.Error("Unable to create TCP ports monitor [%v]", err.Error())
		goto Done
	}

	err = backend.Start()
	if err != nil {
		console.Error("Unable to create server [%v]", err.Error())
		goto Done
	}

Done:
	if err == nil {
		go p.run()
	} else {
		p.shutdown()
	}
	return err
}

func (p *program) Stop(s service.Service) error {
	p.shutdown()
	return nil
}

func (p *program) run()  {
	console.Info("Running server at port %v", settings.Config.Server.Port)

	logger.Run(p.wg)
	processwatcher.Run(p.wg)
	webchecker.Run(p.wg)
	tcpports.Run(p.wg)
	freediskspacechecker.Run(p.wg)
	backend.Run(p.wg)
}

func (p *program) shutdown()  {
	backend.Stop()
	freediskspacechecker.Stop()
	tcpports.Stop()
	webchecker.Stop()
	processwatcher.Stop()
	logger.Stop()

	p.wg.Wait()
}

//------------------------------------------------------------------------------

func main() {
	serviceCmdLineParam, err := process.GetCmdLineParam("service")
	if err != nil {
		console.Error(err.Error())
		return
	}

	svcConfig := &service.Config{
		Name:        "ServerWatchdog",
		DisplayName: "Randlabs.IO Server Watcher service",
		Description: "A service that acts as a centralized notification system and monitors processes, webs and disks.",
	}
	if runtime.GOOS != "windows" {
		svcConfig.Name = "serverwatchdog"
	}

	if serviceCmdLineParam == "install" {
		settingsFilename, err := settings.GetSettingsFilename()
		if err != nil {
			console.Error(err.Error())
			return
		}

		svcConfig.Arguments = append(svcConfig.Arguments, "--settings", settingsFilename)
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		console.Error("Unable to initialize application [%v]", err.Error())
		return
	}
	if len(serviceCmdLineParam) == 0 {
		err = console.SetupService(s)
		if err != nil {
			console.Error("Unable to setup service logger [%v]", err.Error())
			return
		}

		_ = s.Run()
		// no need to print an error message because already printer by the start function
	} else {
		err = service.Control(s, serviceCmdLineParam)
		if err != nil {
			console.Error("Unable to send control code [%v]", err.Error())
			return
		}
	}
	return
}
