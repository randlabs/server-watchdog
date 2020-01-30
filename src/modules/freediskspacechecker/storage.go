package freediskspacechecker

import (
	"sync/atomic"

	"github.com/randlabs/server-watchdog/utils/state"
	"github.com/vmihailenco/msgpack/v4"
)

//------------------------------------------------------------------------------

type FreeDiskSpaceCheckerStateItem struct {
	HashCode         uint64
	LastCheckStatus  bool
}

//------------------------------------------------------------------------------

const (
	freeDiskSpaceCheckerStateFileName = "fdscheck.state"
)

//------------------------------------------------------------------------------

func (m *Module) loadState() error {
	b, err := state.LoadStateBlob(freeDiskSpaceCheckerStateFileName)
	if err == nil && b != nil {
		var loadedItems []FreeDiskSpaceCheckerStateItem

		err = msgpack.Unmarshal(b, &loadedItems)
		if err == nil {
			for idx := range m.devicesList {
				dev := &m.devicesList[idx]

				for _, v := range loadedItems {
					if dev.HashCode == v.HashCode {
						if v.LastCheckStatus {
							atomic.StoreInt32(&dev.LastCheckStatus, 1)
						} else {
							atomic.StoreInt32(&dev.LastCheckStatus, 0)
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
	toSave := make([]FreeDiskSpaceCheckerStateItem, len(m.devicesList))
	for idx, v := range m.devicesList {
		status := false
		if atomic.LoadInt32(&v.LastCheckStatus) != 0 {
			status = true
		}

		toSave[idx] = FreeDiskSpaceCheckerStateItem{
			HashCode        : v.HashCode,
			LastCheckStatus : status,
		}
	}

	b, err := msgpack.Marshal(toSave)
	if err == nil {
		err = state.SaveStateBlob(freeDiskSpaceCheckerStateFileName, b)
	}

	return err
}
