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
	Headers         map[string]string
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

var module *Module
var lock sync.RWMutex

//------------------------------------------------------------------------------

func Start() error {
	//initialize module
	module = &Module{}
	module.shutdownSignal = make(chan struct{})
	module.r.Initialize()

	//build webs list from settings
	module.websList = make([]WebItem, len(settings.Config.Webs))
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

		wh := map[string]string{}
		if web.Headers != nil {
			for key, value := range *web.Headers {
				wh[key] = value
			}
		}

		module.websList[idx] = WebItem{
			h.Sum64(),
			web.Url,
			wh,
			wc,
			web.Channel,
			web.Severity,
			web.CheckPeriodX,
			0,
			1,
			0,
		}
	}

	module.checkDone = make(chan struct{})

	//load stored state
	err := module.loadState()
	if err != nil {
		console.Error("Unable to load web checker state. [%v]", err)
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
				if len(localModule.websList) > 0 {
					var timeToWait time.Duration

					loop := true
					for loop {
						var start time.Time
						var elapsed time.Duration

						//find next web to check
						timeToWait = -1
						for i := len(localModule.websList); i > 0; i-- {
							if atomic.LoadInt32(&localModule.websList[i - 1].CheckInProgress) == 0 {
								if timeToWait < 0 || timeToWait > localModule.websList[i - 1].NextCheckPeriod {
									timeToWait = localModule.websList[i - 1].NextCheckPeriod
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
								localModule.checkWebs(timeToWait)

							case <-localModule.checkDone:
								//if a web check has finished, check again
								elapsed = time.Since(start)
								localModule.checkWebs(elapsed)
							}
						} else {
							select {
							case <-localModule.shutdownSignal:
								loop = false

							case <-localModule.checkDone:
								//if a web check has finished, check for others
								elapsed = time.Since(start)
								localModule.checkWebs(elapsed)
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

func (m *Module) checkWebs(elapsedTime time.Duration) {
	for idx := len(m.websList); idx > 0; idx-- {
		web := &m.websList[idx - 1]

		if atomic.CompareAndSwapInt32(&web.CheckInProgress, 0, 1) {
			if elapsedTime >= web.NextCheckPeriod {
				//reset timer
				web.NextCheckPeriod = web.CheckPeriod

				//check this web
				if m.r.Acquire() {
					go func(web *WebItem) {
						var newStatus int32

						newStatus = 0

						client := http.Client{
							Timeout: 10 * time.Second,
						}

						req, _ := http.NewRequest("GET", web.Url, nil)
						for hdrKey, hdrValue := range web.Headers {
							req.Header.Set(hdrKey, hdrValue)
						}

						resp, err := client.Do(req)
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

											if !allMatches {
												break
											}
										}

										if !allMatches {
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

							m.runSaveState()
						}

						//notify only if status changed from true to false
						if oldStatus == 1 && newStatus == 0 {
							if m.r.Acquire() {
								go func(web *WebItem) {
									_ = logger.Log(web.Severity, web.Channel, "Site '%s' is down.", web.Url)

									m.r.Release()
								}(web)
							}
						}

						atomic.StoreInt32(&web.CheckInProgress, 0)

						select {
						case m.checkDone <- struct{}{}:
						case <-m.shutdownSignal:
						}

						m.r.Release()
					}(web)
				} else {
					atomic.StoreInt32(&web.CheckInProgress, 0)
				}
			} else {
				web.NextCheckPeriod -= elapsedTime

				atomic.StoreInt32(&web.CheckInProgress, 0)
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
				console.Error("Unable to save web checker state. [%v]", err)
			}

			m.r.Release()
		}(m)
	}

	return
}
