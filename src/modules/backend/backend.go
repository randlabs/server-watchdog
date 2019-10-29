package backend

import (
	"github.com/randlabs/server-watchdog/settings"
	"sync"

	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/handlers"
	"github.com/randlabs/server-watchdog/server"
)

//------------------------------------------------------------------------------

type BackendModule struct {
	svr *server.Server
}

//------------------------------------------------------------------------------

var module *BackendModule

//------------------------------------------------------------------------------

func BackendStart() error {
	var err error

	module = &BackendModule{}
	module.svr, err = server.Create(settings.Config.Server.Port, false)
	if err != nil {
		module = nil
		return err
	}
	handlers.Initialize(module.svr.Router)

	console.Info("Running server at port %v", settings.Config.Server.Port)

	return nil
}

func BackendStop() {
	if module != nil {
		module.svr.Stop()

		module = nil
	}

	return
}

func BackendRun(wg sync.WaitGroup) {
	if module != nil {
		wg.Add(1)

		go func() {
			var err error

			err = module.svr.Wait()
			if err != nil {
				console.Error("Server error [%v]", err)
			}
			wg.Done()
		}()
	}

	return
}
