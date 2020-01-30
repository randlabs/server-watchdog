package backend

import (
	"sync"

	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/modules/backend/handlers"
	"github.com/randlabs/server-watchdog/server"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

type Module struct {
	svr *server.Server
}

//------------------------------------------------------------------------------

var module *Module
var lock sync.RWMutex

//------------------------------------------------------------------------------

func Start() error {
	var err error

	module = &Module{}
	module.svr, err = server.Create(settings.Config.Server.Port, false)
	if err != nil {
		module = nil
		return err
	}

	handlers.Initialize(module.svr.Router)

	return nil
}

func Stop() {
	lock.Lock()
	localModule := module
	module = nil
	lock.Unlock()

	if localModule != nil {
		localModule.svr.Stop()
	}

	return
}

func Run(wg sync.WaitGroup) {
	lock.RLock()
	localModule := module
	lock.RUnlock()

	if localModule != nil {
		wg.Add(1)

		go func() {
			err := localModule.svr.Wait()
			if err != nil {
				console.Error("Server error [%v]", err)
			}

			wg.Done()
		}()
	}

	return
}
