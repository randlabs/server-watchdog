package webchecker

import (
	"hash/fnv"
	"io/ioutil"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	rp "github.com/randlabs/rundown-protection"
	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/modules/logger"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

type Module struct {
	shutdownSignal chan struct{}
	websList       []WebItem
	r              rp.RundownProtection
	checkDone      chan struct{}
}

type WebItem struct {
	HashCode        uint64
	Url             string
	Content         []WebItem_Content
	Channel         string
	Severity        string
	CheckPeriod     time.Duration
	NextCheckPeriod time.Duration
	LastCheckStatus int32
	CheckInProgress int32
}

type WebItem_Content struct {
	SearchRegex  *regexp.Regexp
	CheckChanges []uint
	LastContent  []string
}

//------------------------------------------------------------------------------

var webCheckerModule *Module

//------------------------------------------------------------------------------

func Start() error {
	//initialize module
	webCheckerModule = &Module{}
	webCheckerModule.shutdownSignal = make(chan struct{})
	webCheckerModule.r.Initialize()

	//build webs list from settings
	webCheckerModule.websList = make([]WebItem, len(settings.Config.Webs))
	for idx, web := range settings.Config.Webs {
		h := fnv.New64a()
		h.Sum([]byte(web.Url))
		h.Sum([]byte(web.Channel))
		h.Sum([]byte(web.Severity))

		wc := make([]WebItem_Content, len(web.Content))

		for idx, c := range web.Content {
			wc[idx].SearchRegex = c.SearchRegex
			wc[idx].CheckChanges = c.CheckChanges
			wc[idx].LastContent = make([]string, len(c.CheckChanges))

			h.Sum([]byte(c.SearchRegex.String()))
		}

		webCheckerModule.websList[idx] = WebItem{
			h.Sum64(),
			web.Url,
			wc,
			web.Channel,
			web.Severity,
			web.CheckPeriodX,
			0,
			1,
			0,
		}
	}

	webCheckerModule.checkDone = make(chan struct{})

	//load stored state
	err := loadState(webCheckerModule.websList)
	if err != nil {
		console.Error("Unable to load web checker state. [%v]", err)
		return err
	}

	return nil
}

func Stop() {
	if webCheckerModule != nil {
		//signal shutdown
		close(webCheckerModule.shutdownSignal)

		//wait until all workers are done
		webCheckerModule.r.Wait()

		close(webCheckerModule.checkDone)

		webCheckerModule = nil
	}
	return
}

func Run(wg sync.WaitGroup) {
	if webCheckerModule != nil {
		//start background loop
		wg.Add(1)

		if webCheckerModule.r.Acquire() {
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

				webCheckerModule.r.Release()

				wg.Done()
			}()
		} else {
			wg.Done()
		}
	}

	return
}

//------------------------------------------------------------------------------

func (module *Module) checkWebs(elapsedTime time.Duration) {
	for idx := len(module.websList); idx > 0; idx-- {
		if atomic.CompareAndSwapInt32(&module.websList[idx - 1].CheckInProgress, 0, 1) {
			if elapsedTime >= module.websList[idx - 1].NextCheckPeriod {
				//reset timer
				module.websList[idx - 1].NextCheckPeriod = module.websList[idx - 1].CheckPeriod

				//check this web
				if module.r.Acquire() {
					go func(web *WebItem) {
						var newStatus int32

						newStatus = 0

						client := http.Client{
							Timeout: 10 * time.Second,
						}
						resp, err := client.Get(web.Url)
						if err == nil {
							if resp.StatusCode == http.StatusOK {
								if web.Content != nil {
									bodyBytes, err := ioutil.ReadAll(resp.Body)
									if err == nil {
										allMatches := true

										bodyString := string(bodyBytes)

										for contentIdx := range web.Content {
											wc := &web.Content[contentIdx]

											matches := wc.SearchRegex.FindStringSubmatch(bodyString)
											matchesCount := uint(len(matches))
											if matchesCount > 0 {
												matches = matches[1:]
												matchesCount -= 1
											}

											for idx := range wc.CheckChanges {
												matchIndex := wc.CheckChanges[idx] - 1
												if matchIndex >= matchesCount {
													allMatches = false
													break
												}

												if web.Content[contentIdx].LastContent[idx] == matches[matchIndex] {
													break
												}

												web.Content[contentIdx].LastContent[idx] = matches[matchIndex]
												allMatches = false
											}

											if allMatches == false {
												break
											}
										}

										if allMatches == false {
											newStatus = 1
										}
									}


								} else {
									newStatus = 1
								}
							}

							_ = resp.Body.Close()
						}

						oldStatus := atomic.SwapInt32(&web.LastCheckStatus, newStatus)
						if oldStatus != newStatus {
							if newStatus == 0 {
								for contentIdx := range web.Content {
									for idx := range web.Content[contentIdx].LastContent {
										web.Content[contentIdx].LastContent[idx] = ""
									}
								}
							}

							runSaveState()
						}

						//notify only if status changed from true to false
						if oldStatus == 1 && newStatus == 0 {
							if module.r.Acquire() {
								go func(web *WebItem) {
									_ = logger.Log(web.Severity, web.Channel, "Site '%s' is down.", web.Url)

									module.r.Release()
								}(web)
							}
						}

						atomic.StoreInt32(&web.CheckInProgress, 0)

						select {
						case module.checkDone <- struct{}{}:
						case <-module.shutdownSignal:
						}

						module.r.Release()
					}(&module.websList[idx - 1])
				} else {
					atomic.StoreInt32(&module.websList[idx - 1].CheckInProgress, 0)
				}
			} else {
				module.websList[idx - 1].NextCheckPeriod -= elapsedTime

				atomic.StoreInt32(&module.websList[idx - 1].CheckInProgress, 0)
			}
		}
	}

	return
}

func runSaveState() {
	if webCheckerModule.r.Acquire() {
		go func(module *Module) {
			err := saveState(module.websList)

			if err != nil {
				console.Error("Unable to save web checker state. [%v]", err)
			}

			module.r.Release()
		}(webCheckerModule)
	}
	return
}
