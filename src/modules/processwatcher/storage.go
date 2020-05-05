package processwatcher

import (
	"github.com/randlabs/server-watchdog/modules/logger"
	"github.com/randlabs/server-watchdog/utils/state"
	"github.com/vmihailenco/msgpack/v4"
)

//------------------------------------------------------------------------------

type ProcessWatcherStateItem struct {
	Pid            int
	Name           string
	MaxMemUsage    string
	Channel        string
	Severity       string
}

//------------------------------------------------------------------------------

const (
	processWatcherStateFileName = "proceswatcher.state"
)

//------------------------------------------------------------------------------

func (m *Module) loadState() error {
	var loadedItems []ProcessWatcherStateItem

	b, err := state.LoadStateBlob(processWatcherStateFileName)
	if err == nil && b != nil {
		err = msgpack.Unmarshal(b, &loadedItems)
	} else {
		loadedItems = make([]ProcessWatcherStateItem, 0)
	}

	if err == nil {
		var stateModified = false

		for _, v := range loadedItems {
			err = m.addProcessInternal(v.Pid, v.Name, v.MaxMemUsage, v.Severity, v.Channel)
			if err != nil {
				stateModified = true

				if err.Error() == errProcessNotFound {
					if len(v.Name) == 0 {
						_ = logger.Log(v.Severity, v.Channel, "The process #%v has died while the server watcher was down.", v.Pid)
					} else {
						_ = logger.Log(v.Severity, v.Channel, "The process \"%v\" (#%v) has died while the server watcher was down.", v.Name, v.Pid)
					}
				}
			}
		}

		if stateModified {
			m.runSaveState()
		}
	}

	return err
}

func (m *Module) saveState() error {
	toSave := make([]ProcessWatcherStateItem, len(m.processList))
	for idx, v := range m.processList {
		toSave[idx] = ProcessWatcherStateItem{
			Pid             : v.Pid,
			Name            : v.Name,
			MaxMemUsage     : v.MaxMemUsage,
			Channel         : v.Channel,
			Severity        : v.Severity,
		}
	}

	b, err := msgpack.Marshal(toSave)
	if err == nil {
		err = state.SaveStateBlob(processWatcherStateFileName, b)
	}

	return err
}
