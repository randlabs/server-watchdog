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

var fdsCheckerModule *Module

//------------------------------------------------------------------------------

func Start() error {
	//initialize module
	fdsCheckerModule = &Module{}
	fdsCheckerModule.shutdownSignal = make(chan struct{})

	//build devices list from settings
	fdsCheckerModule.devicesList = make([]DeviceItem, len(settings.Config.FreeDiskSpace))
	for idx, fds := range settings.Config.FreeDiskSpace {
		h := fnv.New64a()
		h.Sum([]byte(fds.Device))
		h.Sum([]byte(fds.Channel))
		h.Sum([]byte(fds.Severity))

		fdsCheckerModule.devicesList[idx] = DeviceItem{
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

	fdsCheckerModule.checkDone = make(chan struct{})

	//load stored state
	err := loadState(fdsCheckerModule.devicesList)
	if err != nil {
		console.Error("Unable to load free disk space checker state. [%v]", err)
		return err
	}

	return nil
}

func Stop() {
	if fdsCheckerModule != nil {
		//signal shutdown
		fdsCheckerModule.shutdownSignal <- struct{}{}

		//wait until all workers are done
		fdsCheckerModule.r.Wait()

		close(fdsCheckerModule.checkDone)

		fdsCheckerModule = nil
	}
	return
}

func Run(wg sync.WaitGroup) {
	if fdsCheckerModule != nil {
		//start background loop
		wg.Add(1)

		go func() {
			if len(fdsCheckerModule.devicesList) > 0 {
				var timeToWait time.Duration

				loop := true
				for loop {
					var start time.Time
					var elapsed time.Duration

					//find next device to check
					timeToWait = -1
					for i := len(fdsCheckerModule.devicesList); i > 0; i-- {
						if atomic.LoadInt32(&fdsCheckerModule.devicesList[i - 1].CheckInProgress) == 0 {
							if timeToWait < 0 || timeToWait > fdsCheckerModule.devicesList[i - 1].NextCheckPeriod {
								timeToWait = fdsCheckerModule.devicesList[i - 1].NextCheckPeriod
							}
						}
					}

					start = time.Now()
					if timeToWait >= 0 {
						select {
						case <-fdsCheckerModule.shutdownSignal:
							loop = false

						case <-time.After(timeToWait):
							//check devices when the time to wait elapses
							fdsCheckerModule.checkDevices(timeToWait)

						case <-fdsCheckerModule.checkDone:
							//if a device check has finished, check again
							elapsed = time.Since(start)
							fdsCheckerModule.checkDevices(elapsed)
						}
					} else {
						select {
						case <-fdsCheckerModule.shutdownSignal:
							loop = false

						case <-fdsCheckerModule.checkDone:
							//if a device check has finished, check for others
							elapsed = time.Since(start)
							fdsCheckerModule.checkDevices(elapsed)
						}
					}
				}
			} else {
				<-fdsCheckerModule.shutdownSignal
			}

			wg.Done()
		}()
	}

	return
}

//------------------------------------------------------------------------------

func (module *Module) checkDevices(elapsedTime time.Duration) {
	for idx := len(module.devicesList); idx > 0; idx-- {
		if atomic.CompareAndSwapInt32(&module.devicesList[idx - 1].CheckInProgress, 0, 1) {
			if elapsedTime >= module.devicesList[idx - 1].NextCheckPeriod {
				//reset timer
				module.devicesList[idx - 1].NextCheckPeriod = module.devicesList[idx - 1].CheckPeriod

				//check this device
				if module.r.Acquire() {
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
							runSaveState()
						}

						//notify only if status changed from true to false
						if oldStatus == 1 && newStatus == 0 {
							if module.r.Acquire() {
								go func(dev *DeviceItem) {
									_ = logger.Log(dev.Severity, dev.Channel, "Disk space on '%s' is low.",
										dev.Device)

									module.r.Release()
								}(dev)
							}
						}

						atomic.StoreInt32(&dev.CheckInProgress, 0)

						module.checkDone <- struct{}{}
						module.r.Release()
					}(&module.devicesList[idx - 1])
				} else {
					atomic.StoreInt32(&module.devicesList[idx - 1].CheckInProgress, 0)
				}
			} else {
				module.devicesList[idx - 1].NextCheckPeriod -= elapsedTime

				atomic.StoreInt32(&module.devicesList[idx - 1].CheckInProgress, 0)
			}
		}
	}

	return
}

func runSaveState() {
	if fdsCheckerModule.r.Acquire() {
		go func(module *Module) {
			err := saveState(module.devicesList)

			if err != nil {
				console.Error("Unable to save free disk space checker state. [%v]", err)
			}

			module.r.Release()
		}(fdsCheckerModule)
	}
	return
}
