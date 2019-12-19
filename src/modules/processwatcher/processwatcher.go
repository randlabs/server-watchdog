package processwatcher

import (
	"errors"
	"github.com/randlabs/server-watchdog/console"
	"os"
	"sync"
	"sync/atomic"

	rp "github.com/randlabs/rundown-protection"
	"github.com/randlabs/server-watchdog/modules/logger"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

type Module struct {
	shutdownSignal chan struct{}
	processListMtx sync.Mutex
	processList    []*ProcessItem
	shuttingDown   int32
	r              rp.RundownProtection
}

type ProcessItem struct {
	Pid            int
	Name           string
	Channel        string
	Severity       string
	Proc           *os.Process
	Remove         chan struct{}
	Removing       int32
}

type RunningProcessState struct {
	state *os.ProcessState
	err error
}

//------------------------------------------------------------------------------

const (
	errProcessNotFound = "Process not found"
)

var processWatcherModule *Module

//------------------------------------------------------------------------------

func Start() error {
	//initialize module
	processWatcherModule = &Module{}
	processWatcherModule.shutdownSignal = make(chan struct{})

	//load stored state
	stateItems, err := loadState()
	if err != nil {
		console.Error("Unable to load process watcher state. [%v]", err)
		return err
	}

	var stateModified = false

	for _, v := range stateItems {
		err = addProcessInternal(v.Pid, v.Name, v.Severity, v.Channel)
		if err != nil {
			stateModified = true

			if err.Error() == errProcessNotFound {
				if len(v.Name) == 0 {
					_ = logger.Log(v.Severity, v.Channel, "The process #%v has died while the server watcher was down.", v.Pid)
				} else {
					_ = logger.Log(v.Severity, v.Channel, "The process \"%v\" (#%v) has died while the server watcher was down.", v.Name, v.Pid)
				}
			}
		}
	}

	if stateModified {
		runSaveState()
	}

	return nil
}

func Stop() {
	if processWatcherModule != nil {
		//signal shutdown
		atomic.StoreInt32(&processWatcherModule.shuttingDown, 1)
		processWatcherModule.shutdownSignal <- struct{}{}

		//wait until all workers are done
		processWatcherModule.r.Wait()

		// Because we use buffered channels, this won't deadlock
		processWatcherModule.processListMtx.Lock()
		for i := len(processWatcherModule.processList); i > 0; i-- {
			processWatcherModule.processList[i - 1].Remove <- struct{}{}
		}
		processWatcherModule.processListMtx.Unlock()

		processWatcherModule = nil
	}
	return
}

func Run(wg sync.WaitGroup) {
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

func AddProcess(pid int, name string, severity string, channel string) error {
	err :=  addProcessInternal(pid, name, severity, channel)
	if err == nil {
		runSaveState()
	}
	return err
}

func addProcessInternal(pid int, name string, severity string, channel string) error {
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
			return errors.New(errProcessNotFound)
		}

		p := &ProcessItem{
			Pid: pid,
			Name: name,
			Channel: channel,
			Severity: severity,
			Proc: process,
			Remove: make(chan struct{}, 1),
			Removing: 0,
		}

		processWatcherModule.processList = append(processWatcherModule.processList, p)

		//monitor process
		go func(module *Module, p *ProcessItem) {
			var rps RunningProcessState

			stateChan := make(chan RunningProcessState)

			go func() {
				state, err := p.Proc.Wait()
				stateChan <- RunningProcessState{state, err}
			}()

			select {
			case rps = <- stateChan:

			case <- p.Remove:
				rps = RunningProcessState{
					state: nil,
					err:   errors.New("cancelled"),
				}
			}

			if atomic.LoadInt32(&p.Removing) == 0 {
				//remove from the watch list
				module.processListMtx.Lock()
				defer module.processListMtx.Unlock()

				if atomic.LoadInt32(&p.Removing) == 0 {
					listLen := len(processWatcherModule.processList)

					for i := listLen; i > 0; i-- {
						if processWatcherModule.processList[i - 1].Pid == p.Pid && processWatcherModule.processList[i - 1].Channel == p.Channel {
							//from https://github.com/golang/go/wiki/SliceTricks to avoid leaks
							processWatcherModule.processList[i - 1] = processWatcherModule.processList[listLen - 1]
							processWatcherModule.processList[listLen - 1] = nil
							processWatcherModule.processList = processWatcherModule.processList[:(listLen - 1)]
							break
						}
					}
				}
			}

			if module.r.Acquire() {

				//log terminated process
				if rps.err == nil {
					if !rps.state.Success() {
						if len(p.Name) == 0 {
							_ = logger.Log(p.Severity, p.Channel, "The process #%v has died.", p.Pid)
						} else {
							_ = logger.Log(p.Severity, p.Channel, "The process \"%v\" (#%v) has died.", p.Name, p.Pid)
						}
					}
				} else {
					//should I log if the wait failed?
				}

				module.r.Release()
			}

			_ = p.Proc.Release()
		}(processWatcherModule, p)
	}

	return nil
}

func RemoveProcess(pid int, channel string) {
	var p *ProcessItem = nil

	{
		processWatcherModule.processListMtx.Lock()
		defer processWatcherModule.processListMtx.Unlock()

		listLen := len(processWatcherModule.processList)

		for i := listLen; i > 0; i-- {
			if processWatcherModule.processList[i - 1].Pid == pid && processWatcherModule.processList[i - 1].Channel == channel {
				p = processWatcherModule.processList[i - 1]

				//from https://github.com/golang/go/wiki/SliceTricks to avoid leaks
				processWatcherModule.processList[i - 1] = processWatcherModule.processList[listLen - 1]
				processWatcherModule.processList[listLen - 1] = nil
				processWatcherModule.processList = processWatcherModule.processList[:(listLen - 1)]
				break
			}
		}
	}

	if p != nil {
		atomic.StoreInt32(&p.Removing, 1)
		p.Remove <- struct{}{}

		runSaveState()
	}
	return
}

func runSaveState() {
	if processWatcherModule.r.Acquire() {
		go func(module *Module) {
			module.processListMtx.Lock()
			err := saveState(module.processList)
			module.processListMtx.Unlock()

			if err != nil {
				console.Error("Unable to save process watcher state. [%v]", err)
			}

			module.r.Release()
		}(processWatcherModule)
	}
	return
}
