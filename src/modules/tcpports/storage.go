package tcpports

import (
	"github.com/randlabs/server-watchdog/utils/state"
	"github.com/vmihailenco/msgpack/v4"
)

//------------------------------------------------------------------------------

type TcpPortsCheckerStateItem struct {
	HashCode uint64
	Ports    []TcpPortsCheckerStateItem_Port
}

type TcpPortsCheckerStateItem_Port struct {
	Port             uint32
	LastCheckStatus  bool
}

//------------------------------------------------------------------------------

const (
	tcpPortsCheckerStateFileName = "tcpportscheck.state"
)

//------------------------------------------------------------------------------

func (m *Module) loadState() error {
	b, err := state.LoadStateBlob(tcpPortsCheckerStateFileName)
	if err == nil && b != nil {
		var loadedItems []TcpPortsCheckerStateItem

		err = msgpack.Unmarshal(b, &loadedItems)
		if err == nil {
			for idx := range m.tcpPortsList {
				port := &m.tcpPortsList[idx]

				for _, v := range loadedItems {
					if port.HashCode == v.HashCode {
						port.LastCheckStatusLock.Lock()

						for _, loadedItemPort := range v.Ports {
							if loadedItemPort.Port <= 65535 {
								if loadedItemPort.LastCheckStatus {
									port.LastCheckStatus.Add(loadedItemPort.Port)
								} else {
									port.LastCheckStatus.Remove(loadedItemPort.Port)
								}
							}
						}

						port.LastCheckStatusLock.Unlock()
						break
					}
				}
			}
		}
	}

	return err
}

func (m *Module) saveState() error {
	toSave := make([]TcpPortsCheckerStateItem, len(m.tcpPortsList))
	for idx, v := range m.tcpPortsList {
		toSave[idx] = TcpPortsCheckerStateItem{
			HashCode : v.HashCode,
			Ports    : make([]TcpPortsCheckerStateItem_Port, v.PortsX.GetCardinality()),
		}

		pIdx := 0
		v.LastCheckStatusLock.Lock()
		it := v.PortsX.Iterator()
		for it.HasNext() {
			vPort := &toSave[idx].Ports[pIdx]

			vPort.Port = it.Next()

			if v.LastCheckStatus.Contains(vPort.Port) {
				vPort.LastCheckStatus = true
			} else {
				vPort.LastCheckStatus = true
			}
		}
		v.LastCheckStatusLock.Unlock()
	}

	b, err := msgpack.Marshal(toSave)
	if err == nil {
		err = state.SaveStateBlob(tcpPortsCheckerStateFileName, b)
	}

	return err
}
