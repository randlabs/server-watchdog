package webchecker

import (
	"sync/atomic"

	"github.com/randlabs/server-watchdog/utils/state"
	"github.com/vmihailenco/msgpack/v4"
)

//------------------------------------------------------------------------------

type WebCheckerStateItem struct {
	HashCode         uint64
	LastCheckStatus  bool
}

//------------------------------------------------------------------------------

const (
	webCheckerStateFileName = "webcheck.state"
)

//------------------------------------------------------------------------------

func (m *Module) loadState() error {
	b, err := state.LoadStateBlob(webCheckerStateFileName)
	if err == nil && b != nil {
		var loadedItems []WebCheckerStateItem

		err = msgpack.Unmarshal(b, &loadedItems)
		if err == nil {
			for idx := range m.websList {
				web := &m.websList[idx]

				for _, v := range loadedItems {
					if web.HashCode == v.HashCode {
						if v.LastCheckStatus {
							atomic.StoreInt32(&web.LastCheckStatus, 1)
						} else {
							atomic.StoreInt32(&web.LastCheckStatus, 0)
						}
						break
					}
				}
			}
		}
	}

	return err
}

func (m *Module) saveState() error {
	toSave := make([]WebCheckerStateItem, len(m.websList))
	for idx, v := range m.websList {
		status := false
		if atomic.LoadInt32(&v.LastCheckStatus) != 0 {
			status = true
		}

		toSave[idx] = WebCheckerStateItem{
			HashCode        : v.HashCode,
			LastCheckStatus : status,
		}
	}

	b, err := msgpack.Marshal(toSave)
	if err == nil {
		err = state.SaveStateBlob(webCheckerStateFileName, b)
	}

	return err
}
