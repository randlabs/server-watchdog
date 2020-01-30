package freediskspacechecker

import (
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	rp "github.com/randlabs/rundown-protection"
	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/modules/logger"
	"github.com/randlabs/server-watchdog/settings"
	"github.com/ricochet2200/go-disk-usage/du"
)

//------------------------------------------------------------------------------

type Module struct {
	shutdownSignal chan struct{}
	devicesList    []DeviceItem
	r              rp.RundownProtection
	checkDone      chan struct{}
}

type DeviceItem struct {
	HashCode         uint64
	Device           string
	Channel          string
	Severity         string
	MinimumFreeSpace uint64
	CheckPeriod      time.Duration
	NextCheckPeriod  time.Duration
	LastCheckStatus  int32
	CheckInProgress  int32
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

	//build devices list from settings
	module.devicesList = make([]DeviceItem, len(settings.Config.FreeDiskSpace))
	for idx, fds := range settings.Config.FreeDiskSpace {
		h := fnv.New64a()
		h.Sum([]byte(fds.Device))
		h.Sum([]byte(fds.Channel))
		h.Sum([]byte(fds.Severity))

		module.devicesList[idx] = DeviceItem{
			h.Sum64(),
			fds.Device,
			fds.Channel,
			fds.Severity,
			fds.MinimumSpaceX,
			fds.CheckPeriodX,
			0,
			1,
			0,
		}
	}

	module.checkDone = make(chan struct{})

	//load stored state
	err := module.loadState()
	if err != nil {
		console.Error("Unable to load free disk space checker state. [%v]", err)
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
				if len(localModule.devicesList) > 0 {
					var timeToWait time.Duration

					loop := true
					for loop {
						var start time.Time
						var elapsed time.Duration

						//find next device to check
						timeToWait = -1
						for i := len(localModule.devicesList); i > 0; i-- {
							if atomic.LoadInt32(&localModule.devicesList[i-1].CheckInProgress) == 0 {
								if timeToWait < 0 || timeToWait > localModule.devicesList[i-1].NextCheckPeriod {
									timeToWait = localModule.devicesList[i-1].NextCheckPeriod
								}
							}
						}

						start = time.Now()
						if timeToWait >= 0 {
							select {
							case <-localModule.shutdownSignal:
								loop = false

							case <-time.After(timeToWait):
								//check devices when the time to wait elapses
								localModule.checkDevices(timeToWait)

							case <-localModule.checkDone:
								//if a device check has finished, check again
								elapsed = time.Since(start)
								localModule.checkDevices(elapsed)
							}
						} else {
							select {
							case <-localModule.shutdownSignal:
								loop = false

							case <-localModule.checkDone:
								//if a device check has finished, check for others
								elapsed = time.Since(start)
								localModule.checkDevices(elapsed)
							}
						}
					}
				} else {
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

func (m *Module) checkDevices(elapsedTime time.Duration) {
	for idx := len(m.devicesList); idx > 0; idx-- {
		dev := &m.devicesList[idx - 1]

		if atomic.CompareAndSwapInt32(&dev.CheckInProgress, 0, 1) {
			if elapsedTime >= dev.NextCheckPeriod {
				//reset timer
				dev.NextCheckPeriod = dev.CheckPeriod

				//check this device
				if m.r.Acquire() {
					go func(dev *DeviceItem) {
						var newStatus int32

						usage := du.NewDiskUsage(dev.Device)

						if usage.Free() >= dev.MinimumFreeSpace {
							newStatus = 1
						} else {
							newStatus = 0
						}

						oldStatus := atomic.SwapInt32(&dev.LastCheckStatus, newStatus)
						if oldStatus != newStatus {
							m.runSaveState()
						}

						//notify only if status changed from true to false
						if oldStatus == 1 && newStatus == 0 {
							if m.r.Acquire() {
								go func(dev *DeviceItem) {
									_ = logger.Log(dev.Severity, dev.Channel, "Disk space on '%s' is low.",
										dev.Device)

									m.r.Release()
								}(dev)
							}
						}

						atomic.StoreInt32(&dev.CheckInProgress, 0)

						select {
							case m.checkDone <- struct{}{}:
							case <-m.shutdownSignal:
						}

						m.r.Release()
					}(dev)
				} else {
					atomic.StoreInt32(&dev.CheckInProgress, 0)
				}
			} else {
				dev.NextCheckPeriod -= elapsedTime

				atomic.StoreInt32(&dev.CheckInProgress, 0)
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
				console.Error("Unable to save free disk space checker state. [%v]", err)
			}

			m.r.Release()
		}(m)
	}
	return
}
