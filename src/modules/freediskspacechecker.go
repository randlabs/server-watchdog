package modules

import (
	"github.com/randlabs/server-watchdog/modules/logger"
	"sync"
	"sync/atomic"
	"time"

	rp "github.com/randlabs/rundown-protection"
	"github.com/randlabs/server-watchdog/settings"
	"github.com/ricochet2200/go-disk-usage/du"
)

//------------------------------------------------------------------------------

type FreeDiskSpaceCheckerModule struct {
	shutdownSignal chan bool
	devicesList []DeviceItem
	r rp.RundownProtection
	checkDone chan struct{}
}

type DeviceItem struct {
	Device           string
	Channel          string
	Severity         string
	MinimumFreeSpace uint64
	CheckPeriod      time.Duration
	NextCheckPeriod  time.Duration
	LastCheckStatus  bool
	CheckInProgress  int32
}

//------------------------------------------------------------------------------

var fdsCheckerModule *FreeDiskSpaceCheckerModule

//------------------------------------------------------------------------------

func FreeDiskSpaceCheckerStart() error {
	//initialize module
	fdsCheckerModule = &FreeDiskSpaceCheckerModule{}
	fdsCheckerModule.shutdownSignal = make(chan bool, 1)

	//build devices list from settings
	fdsCheckerModule.devicesList = make([]DeviceItem, len(settings.Config.FreeDiskSpace))
	for idx, fds := range settings.Config.FreeDiskSpace {
		fdsCheckerModule.devicesList[idx] = DeviceItem{
			fds.Device, fds.Channel, fds.Severity, fds.MinimumSpaceX, fds.CheckPeriodX, 0, true, 0,
		}
	}

	fdsCheckerModule.checkDone = make(chan struct{})

	return nil
}

func FreeDiskSpaceCheckerStop() {
	if fdsCheckerModule != nil {
		//signal shutdown
		fdsCheckerModule.shutdownSignal <- true

		//wait until all workers are done
		fdsCheckerModule.r.Wait()

		close(fdsCheckerModule.checkDone)

		fdsCheckerModule = nil
	}
	return
}

func FreeDiskSpaceCheckerRun(wg sync.WaitGroup) {
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

func (module *FreeDiskSpaceCheckerModule) checkDevices(elapsedTime time.Duration) {
	for idx := len(module.devicesList); idx > 0; idx-- {
		if atomic.LoadInt32(&module.devicesList[idx - 1].CheckInProgress) == 0 {
			if elapsedTime >= module.devicesList[idx - 1].NextCheckPeriod {
				//reset timer
				module.devicesList[idx - 1].NextCheckPeriod = module.devicesList[idx - 1].CheckPeriod

				//check this device
				if module.r.Acquire() {
					go func(dev *DeviceItem) {
						usage := du.NewDiskUsage(dev.Device)

						if usage.Free() < dev.MinimumFreeSpace {
							if dev.LastCheckStatus != false {
								dev.LastCheckStatus = false

								if module.r.Acquire() {
									go func(dev *DeviceItem) {
										_ = logger.Log(dev.Severity, dev.Channel, "The disk space on '%s' is low.",
														dev.Device)

										module.r.Release()
									}(dev)
								}
							}
						} else {
							dev.LastCheckStatus = true
						}

						module.checkDone <- struct{}{}
						module.r.Release()
					}(&module.devicesList[idx - 1])
				} else {
					module.devicesList[idx - 1].NextCheckPeriod -= elapsedTime
				}
			}
		}
	}

	return
}