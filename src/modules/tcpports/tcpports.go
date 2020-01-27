package tcpports

import (
	"fmt"
	"github.com/RoaringBitmap/roaring"
	"github.com/randlabs/server-watchdog/modules/logger"
	"hash/fnv"
	"net"
	"sync"
	"sync/atomic"
	"time"

	rp "github.com/randlabs/rundown-protection"
	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

type Module struct {
	shutdownSignal chan struct{}
	tcpPortsList   []TcpPortItem
	r              rp.RundownProtection
	checkDone      chan struct{}
}

type TcpPortItem struct {
	HashCode            uint64
	Name                string
	Address             string
	PortsX              *roaring.Bitmap
	Channel             string
	Severity            string
	CheckPeriod         time.Duration
	NextCheckPeriod     time.Duration
	LastCheckStatusLock sync.Mutex
	LastCheckStatus     *roaring.Bitmap
	CheckInProgress     int32
}

//------------------------------------------------------------------------------

var module *Module
var lock sync.RWMutex

//------------------------------------------------------------------------------

func Start() error {
	//initialize module
	module = &Module{}
	module.shutdownSignal = make(chan struct{})
	module.r.Initialize()

	//build tcp ports list from settings
	module.tcpPortsList = make([]TcpPortItem, len(settings.Config.TcpPorts))
	for idx, port := range settings.Config.TcpPorts {
		h := fnv.New64a()
		h.Sum([]byte(port.Name))
		h.Sum([]byte(port.PortsX.String()))
		h.Sum([]byte(port.Channel))
		h.Sum([]byte(port.Severity))

		module.tcpPortsList[idx] = TcpPortItem{
			h.Sum64(),
			port.Name,
			port.Address,
			port.PortsX,
			port.Channel,
			port.Severity,
			port.CheckPeriodX,
			0,
			sync.Mutex{},
			roaring.New(),
			0,
		}
	}

	module.checkDone = make(chan struct{})

	//load stored state
	err := module.loadState()
	if err != nil {
		console.Error("Unable to load tcp ports checker state. [%v]", err)
		return err
	}

	return nil
}

func Stop() {
	lock.Lock()
	localModule := module
	module = nil
	lock.Unlock()

	if localModule != nil {
		//signal shutdown
		close(localModule.shutdownSignal)

		//wait until all workers are done
		localModule.r.Wait()

		close(localModule.checkDone)
	}
	return
}

func Run(wg sync.WaitGroup) {
	lock.RLock()
	localModule := module
	lock.RUnlock()

	if localModule != nil {
		//start background loop
		wg.Add(1)

		if localModule.r.Acquire() {
			go func() {
				if len(localModule.tcpPortsList) > 0 {
					var timeToWait time.Duration

					loop := true
					for loop {
						var start time.Time
						var elapsed time.Duration

						//find next web to check
						timeToWait = -1
						for i := len(localModule.tcpPortsList); i > 0; i-- {
							if atomic.LoadInt32(&localModule.tcpPortsList[i - 1].CheckInProgress) == 0 {
								if timeToWait < 0 || timeToWait > localModule.tcpPortsList[i - 1].NextCheckPeriod {
									timeToWait = localModule.tcpPortsList[i - 1].NextCheckPeriod
								}
							}
						}

						start = time.Now()
						if timeToWait >= 0 {
							select {
							case <-localModule.shutdownSignal:
								loop = false

							case <-time.After(timeToWait):
								//check webs when the time to wait elapses
								localModule.checkTcpPorts(timeToWait)

							case <-localModule.checkDone:
								//if a web check has finished, check again
								elapsed = time.Since(start)
								localModule.checkTcpPorts(elapsed)
							}
						} else {
							select {
							case <-localModule.shutdownSignal:
								loop = false

							case <-localModule.checkDone:
								//if a web check has finished, check for others
								elapsed = time.Since(start)
								localModule.checkTcpPorts(elapsed)
							}
						}
					}
				} else {
					//no webs to check, just wait for the shutdown signal
					<-localModule.shutdownSignal
				}

				localModule.r.Release()

				wg.Done()
			}()
		} else {
			wg.Done()
		}
	}

	return
}

//------------------------------------------------------------------------------

func (m *Module) checkTcpPorts(elapsedTime time.Duration) {
	for idx := len(m.tcpPortsList); idx > 0; idx-- {
		port := &m.tcpPortsList[idx - 1]

		if atomic.CompareAndSwapInt32(&port.CheckInProgress, 0, 1) {
			if elapsedTime >= port.NextCheckPeriod {
				//reset timer
				port.NextCheckPeriod = port.CheckPeriod

				//check this tcp port set
				if m.r.Acquire() {
					go func(port *TcpPortItem) {
						status := make([]bool, port.PortsX.GetCardinality())
						wg := sync.WaitGroup{}

						pIdx := 0
						it := port.PortsX.Iterator()
						for it.HasNext() {
							portNum := it.Next()

							wg.Add(1)

							go func(port *TcpPortItem, portNum uint32, pIdx int) {
								conn, err := net.DialTimeout("tcp", net.JoinHostPort(port.Address, fmt.Sprint(portNum)), 5 * time.Second)
								if err == nil {
									status[pIdx] = true
								} else {
									status[pIdx] = false
								}
								if conn != nil {
									defer conn.Close()
								}

								wg.Done()
							}(port, portNum, pIdx)

							pIdx++

							wg.Wait()
						}

						dropDetected := false
						doSave := false

						port.LastCheckStatusLock.Lock()

						pIdx = 0
						it = port.PortsX.Iterator()
						for it.HasNext() {
							portNum := it.Next()

							if status[pIdx] {
								if !port.LastCheckStatus.Contains(portNum) {
									doSave = true
									port.LastCheckStatus.Add(portNum)
								}
							} else {
								if port.LastCheckStatus.Contains(portNum) {
									dropDetected = true
									doSave = true
									port.LastCheckStatus.Remove(portNum)
								}
							}

							pIdx++
						}

						port.LastCheckStatusLock.Unlock()

						if doSave {
							m.runSaveState()
						}

						//notify only if status changed from true to false
						if dropDetected {
							if m.r.Acquire() {
								go func(port *TcpPortItem) {
									_ = logger.Log(port.Severity, port.Channel, "TCP Ports of group '%s' are down.", port.Name)

									m.r.Release()
								}(port)
							}
						}

						atomic.StoreInt32(&port.CheckInProgress, 0)

						select {
						case m.checkDone <- struct{}{}:
						case <-m.shutdownSignal:
						}

						m.r.Release()
					}(port)
				} else {
					atomic.StoreInt32(&port.CheckInProgress, 0)
				}
			} else {
				port.NextCheckPeriod -= elapsedTime

				atomic.StoreInt32(&port.CheckInProgress, 0)
			}
		}
	}

	return
}

func (m *Module) runSaveState() {
	if m.r.Acquire() {
		go func(m *Module) {
			err := m.saveState()

			if err != nil {
				console.Error("Unable to save tcp ports checker state. [%v]", err)
			}

			m.r.Release()
		}(m)
	}

	return
}
