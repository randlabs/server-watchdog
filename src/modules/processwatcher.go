package modules

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/mitchellh/go-ps"
	rp "github.com/randlabs/rundown-protection"
	"github.com/randlabs/server-watchdog/modules/logger"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

type ProcessWatcherModule struct {
	shutdownSignal chan bool
	processListMtx sync.RWMutex
	processList []ProcessItem
	r rp.RundownProtection
}

type ProcessItem struct {
	Pid      int
	Name     string
	Channel  string
	Severity string
}

//------------------------------------------------------------------------------

var processWatcherModule *ProcessWatcherModule

//------------------------------------------------------------------------------

func ProcessWatcherStart() error {
	//initialize module
	processWatcherModule = &ProcessWatcherModule{}
	processWatcherModule.shutdownSignal = make(chan bool, 1)

	return nil
}

func ProcessWatcherStop() {
	if processWatcherModule != nil {
		//signal shutdown
		processWatcherModule.shutdownSignal <- true

		//wait until all workers are done
		processWatcherModule.r.Wait()

		processWatcherModule = nil
	}
	return
}

func ProcessWatcherRun(wg sync.WaitGroup) {
	if processWatcherModule != nil {
		//start background loop
		wg.Add(1)

		go func() {
			loop := true
			for loop {
				select {
				case <-processWatcherModule.shutdownSignal:
					loop = false

				case <-time.After(10 * time.Second):
					//check for running processes each 10 seconds
					processWatcherModule.checkProcesses()
				}
			}

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
		processWatcherModule.processList = append(processWatcherModule.processList, ProcessItem{
			pid,
			name,
			channel,
			severity,
		})
	}

	return nil
}

func ProcessWatcherRemove(pid int, channel string) {
	processWatcherModule.processListMtx.Lock()
	defer processWatcherModule.processListMtx.Unlock()

	for i := len(processWatcherModule.processList); i > 0; i-- {
		if processWatcherModule.processList[i - 1].Pid == pid && processWatcherModule.processList[i - 1].Channel == channel {
			listLen := len(processWatcherModule.processList)

			processWatcherModule.processList[i - 1] = processWatcherModule.processList[listLen - 1]
			processWatcherModule.processList[listLen - 1] = ProcessItem{0, "", "", ""}
			processWatcherModule.processList = processWatcherModule.processList[:(listLen - 1)]
			break
		}
	}

	return
}

//------------------------------------------------------------------------------

func (module *ProcessWatcherModule) checkProcesses() {
	//get the list of processes
	processes, err := ps.Processes()
	if err == nil {
		var pids []int

		pids = make([]int, len(processes))
		for i, p := range processes {
			pids[i] = p.Pid()
		}
		sort.Ints(pids)

		module.processListMtx.RLock()
		defer module.processListMtx.RUnlock()

		//check if some of the processes has ended
		for i := len(module.processList); i > 0; i-- {
			idx := sort.SearchInts(pids, module.processList[i - 1].Pid)
			if idx >= len(pids) || pids[idx] != module.processList[i - 1].Pid {
				//log terminated process
				if module.r.Acquire() {
					go func(p ProcessItem) {
						if len(p.Name) == 0 {
							_ = logger.Log(p.Severity, p.Channel, "The process #%v has died.", p.Pid)
						} else {
							_ = logger.Log(p.Severity, p.Channel, "The process \"%v\" (#%v) has died.", p.Name, p.Pid)
						}

						module.r.Release()
					}(module.processList[i - 1])
				}

				//and remove from the watch list
				listLen := len(module.processList)

				module.processList[i - 1] = module.processList[listLen - 1]
				module.processList[listLen - 1] = ProcessItem{0, "", "", ""}
				module.processList = module.processList[:(listLen - 1)]
			}
		}
	}
	return
}
