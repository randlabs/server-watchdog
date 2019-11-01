package modules

import (
	"github.com/randlabs/server-watchdog/modules/logger"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	rp "github.com/randlabs/rundown-protection"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

type WebCheckerModule struct {
	shutdownSignal chan struct{}
	websList       []WebItem
	r              rp.RundownProtection
	checkDone      chan struct{}
}

type WebItem struct {
	Url             string
	Channel         string
	Severity        string
	CheckPeriod     time.Duration
	NextCheckPeriod time.Duration
	LastCheckStatus bool
	CheckInProgress int32
}

//------------------------------------------------------------------------------

var webCheckerModule *WebCheckerModule

//------------------------------------------------------------------------------

func WebCheckerStart() error {
	//initialize module
	webCheckerModule = &WebCheckerModule{}
	webCheckerModule.shutdownSignal = make(chan struct{})

	//build webs list from settings
	webCheckerModule.websList = make([]WebItem, len(settings.Config.Webs))
	for idx, web := range settings.Config.Webs {
		webCheckerModule.websList[idx] = WebItem{
			web.Url, web.Channel, web.Severity, web.CheckPeriodX, 0, true, 0,
		}
	}

	webCheckerModule.checkDone = make(chan struct{})

	return nil
}

func WebCheckerStop() {
	if webCheckerModule != nil {
		//signal shutdown
		webCheckerModule.shutdownSignal <- struct{}{}

		//wait until all workers are done
		webCheckerModule.r.Wait()

		close(webCheckerModule.checkDone)

		webCheckerModule = nil
	}
	return
}

func WebCheckerRun(wg sync.WaitGroup) {
	if webCheckerModule != nil {
		//start background loop
		wg.Add(1)

		go func() {
			if len(webCheckerModule.websList) > 0 {
				var timeToWait time.Duration

				loop := true
				for loop {
					var start time.Time
					var elapsed time.Duration

					//find next web to check
					timeToWait = -1
					for i := len(webCheckerModule.websList); i > 0; i-- {
						if atomic.LoadInt32(&webCheckerModule.websList[i - 1].CheckInProgress) == 0 {
							if timeToWait < 0 || timeToWait > webCheckerModule.websList[i - 1].NextCheckPeriod {
								timeToWait = webCheckerModule.websList[i - 1].NextCheckPeriod
							}
						}
					}

					start = time.Now()
					if timeToWait >= 0 {
						select {
						case <-webCheckerModule.shutdownSignal:
							loop = false

						case <-time.After(timeToWait):
							//check webs when the time to wait elapses
							webCheckerModule.checkWebs(timeToWait)

						case <-webCheckerModule.checkDone:
							//if a web check has finished, check again
							elapsed = time.Since(start)
							webCheckerModule.checkWebs(elapsed)
						}
					} else {
						select {
						case <-webCheckerModule.shutdownSignal:
							loop = false

						case <-webCheckerModule.checkDone:
							//if a web check has finished, check for others
							elapsed = time.Since(start)
							webCheckerModule.checkWebs(elapsed)
						}
					}
				}
			} else {
				//no webs to check, just wait for the shutdown signal
				<-webCheckerModule.shutdownSignal
			}

			wg.Done()
		}()
	}

	return
}

//------------------------------------------------------------------------------

func (module *WebCheckerModule) checkWebs(elapsedTime time.Duration) {
	for idx := len(module.websList); idx > 0; idx-- {
		if atomic.LoadInt32(&module.websList[idx - 1].CheckInProgress) == 0 {
			if elapsedTime >= module.websList[idx - 1].NextCheckPeriod {
				//reset timer
				module.websList[idx - 1].NextCheckPeriod = module.websList[idx - 1].CheckPeriod

				atomic.StoreInt32(&module.websList[idx - 1].CheckInProgress, 1)

				//check this web
				if module.r.Acquire() {
					go func(web *WebItem) {
						notify := false

						client := http.Client{
							Timeout: 10 * time.Second,
						}
						resp, err := client.Get(web.Url)
						if err != nil {
							if web.LastCheckStatus != false {
								notify = true
							}
							web.LastCheckStatus = false
						} else {
							if resp.StatusCode != 200 {
								if web.LastCheckStatus != false {
									notify = true
								}
							} else {
								web.LastCheckStatus = true
							}

							resp.Body.Close()
						}

						if notify {
							if module.r.Acquire() {
								go func(web *WebItem) {
									_ = logger.Log(web.Severity, web.Channel, "Site '%s' is down.", web.Url)

									module.r.Release()
								}(web)
							}
						}

						module.checkDone <- struct{}{}
						module.r.Release()
					}(&module.websList[idx - 1])
				}
			} else {
				module.websList[idx - 1].NextCheckPeriod -= elapsedTime
			}
		}
	}

	return
}
