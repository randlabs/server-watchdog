package processwatcher

import (
	"github.com/randlabs/server-watchdog/utils/state"
	"github.com/vmihailenco/msgpack/v4"
)

//------------------------------------------------------------------------------

type ProcessWatcherStateItem struct {
	Pid            int
	Name           string
	Channel        string
	Severity       string
}

//------------------------------------------------------------------------------

const (
	processWatcherStateFileName = "proceswatcher.state"
)

//------------------------------------------------------------------------------

func loadState() ([]ProcessWatcherStateItem, error) {
	var loaded []ProcessWatcherStateItem

	b, err := state.LoadStateBlob(processWatcherStateFileName)
	if err == nil && b != nil {
		err = msgpack.Unmarshal(b, &loaded)
	} else {
		loaded = make([]ProcessWatcherStateItem, 0)
	}

	return loaded, err
}

func saveState(items []*ProcessItem) error {
	toSave := make([]ProcessWatcherStateItem, len(items))
	for idx, v := range items {
		toSave[idx] = ProcessWatcherStateItem{
			Pid             : v.Pid,
			Name            : v.Name,
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
