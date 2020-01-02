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

func loadState(items []DeviceItem) error {
	b, err := state.LoadStateBlob(freeDiskSpaceCheckerStateFileName)
	if err == nil && b != nil {
		var loadedItems []FreeDiskSpaceCheckerStateItem

		err = msgpack.Unmarshal(b, &loadedItems)
		if err == nil {
			for idx, v := range items {
				for _, v2 := range loadedItems {
					if v.HashCode == v2.HashCode {
						if v2.LastCheckStatus {
							atomic.StoreInt32(&items[idx].LastCheckStatus, 1)
						} else {
							atomic.StoreInt32(&items[idx].LastCheckStatus, 0)
						}
						break
					}
				}
			}
		}
	}
	return err
}

func saveState(items []DeviceItem) error {
	toSave := make([]FreeDiskSpaceCheckerStateItem, len(items))
	for idx, v := range items {
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
