package modules

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"

	rp "github.com/randlabs/rundown-protection"
	"github.com/randlabs/server-watchdog/modules/logger"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

type ProcessWatcherModule struct {
	shutdownSignal chan struct{}
	processListMtx sync.Mutex
	processList    []ProcessItem
	r              rp.RundownProtection
}

type ProcessItem struct {
	Pid            int
	Name           string
	Channel        string
	Severity       string
	Proc           *os.Process
	Removed        int32
}

//------------------------------------------------------------------------------

var processWatcherModule *ProcessWatcherModule

//------------------------------------------------------------------------------

func ProcessWatcherStart() error {
	//initialize module
	processWatcherModule = &ProcessWatcherModule{}
	processWatcherModule.shutdownSignal = make(chan struct{})

	return nil
}

func ProcessWatcherStop() {
	if processWatcherModule != nil {
		//signal shutdown
		processWatcherModule.shutdownSignal <- struct{}{}

		//wait until all workers are done
		processWatcherModule.r.Wait()

		for i := len(processWatcherModule.processList); i > 0; i-- {
			atomic.StoreInt32(&processWatcherModule.processList[i - 1].Removed, 1)
		}

		processWatcherModule = nil
	}
	return
}

func ProcessWatcherRun(wg sync.WaitGroup) {
	if processWatcherModule != nil {
		//start background loop
		wg.Add(1)

		go func() {
			<-processWatcherModule.shutdownSignal

			wg.Done()
		}()
	}

	return
}

func ProcessWatcherAdd(pid int, name string, severity string, channel string) error {
	var i int

	severity = settings.ValidateSeverity(severity)
	if len(severity) == 0 {
		return errors.New("Invalid type")
	}

	processWatcherModule.processListMtx.Lock()
	defer processWatcherModule.processListMtx.Unlock()

	for i = len(processWatcherModule.processList); i > 0; i-- {
		if processWatcherModule.processList[i - 1].Pid == pid && processWatcherModule.processList[i - 1].Channel == channel {
			break
		}
	}
	if i == 0 {
		process, err := os.FindProcess(pid)
		if err != nil {
			return errors.New("Process not found")
		}

		p := ProcessItem{
			Pid: pid,
			Name: name,
			Channel: channel,
			Severity: severity,
			Proc: process,
			Removed: 0,
		}

		processWatcherModule.processList = append(processWatcherModule.processList, p)

		//monitor process
		go func(module *ProcessWatcherModule, p *ProcessItem) {
			state, err := p.Proc.Wait()

			if atomic.LoadInt32(&p.Removed) == 0 {
				if module.r.Acquire() {

					//log terminated process
					if err == nil {
						if !state.Success() {
							if len(p.Name) == 0 {
								_ = logger.Log(p.Severity, p.Channel, "The process #%v has died.", p.Pid)
							} else {
								_ = logger.Log(p.Severity, p.Channel, "The process \"%v\" (#%v) has died.", p.Name, p.Pid)
							}
						}
					} else {
						//should I log if the wait failed?
					}

					//remove from the watch list
					module.processListMtx.Lock()

					//look the item in the list
					listLen := len(module.processList)
					for i := 0; i < listLen; i++ {
						if p.Proc == module.processList[i].Proc {
							//found it

							//from https://github.com/golang/go/wiki/SliceTricks to avoid leaks
							processWatcherModule.processList[i] = processWatcherModule.processList[listLen-1]
							processWatcherModule.processList[listLen-1] = ProcessItem{0, "", "", "", nil, 0}
							processWatcherModule.processList = processWatcherModule.processList[:(listLen - 1)]

							break
						}
					}

					module.processListMtx.Unlock()

					module.r.Release()
				}
			}

			_ = p.Proc.Release()
		}(processWatcherModule, &p)
	}

	return nil
}

func ProcessWatcherRemove(pid int, channel string) {
	processWatcherModule.processListMtx.Lock()
	defer processWatcherModule.processListMtx.Unlock()

	for i := len(processWatcherModule.processList); i > 0; i-- {
		if processWatcherModule.processList[i - 1].Pid == pid && processWatcherModule.processList[i - 1].Channel == channel {
			listLen := len(processWatcherModule.processList)

			atomic.StoreInt32(&processWatcherModule.processList[i - 1].Removed, 1)

			//from https://github.com/golang/go/wiki/SliceTricks to avoid leaks
			processWatcherModule.processList[i - 1] = processWatcherModule.processList[listLen - 1]
			processWatcherModule.processList[listLen - 1] = ProcessItem{0, "", "", "", nil, 0}
			processWatcherModule.processList = processWatcherModule.processList[:(listLen - 1)]
			break
		}
	}

	return
}
