package processwatcher

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar"
	rp "github.com/randlabs/rundown-protection"
	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/modules/logger"
	"github.com/randlabs/server-watchdog/settings"
	gops_proc "github.com/shirou/gopsutil/process"
)

//------------------------------------------------------------------------------

type Module struct {
	shutdownSignal chan struct{}
	processListMtx sync.Mutex
	processList    []*ProcessItem
	r              rp.RundownProtection
}

type ProcessItem struct {
	Pid            int
	Name           string
	Channel        string
	Severity       string
}

//------------------------------------------------------------------------------

const (
	errProcessNotFound = "Process not found"
)

var module *Module
var lock sync.RWMutex

//------------------------------------------------------------------------------

func Start() error {
	//initialize module
	module = &Module{}
	module.shutdownSignal = make(chan struct{})
	module.r.Initialize()

	//load stored state
	err := module.loadState()
	if err != nil {
		console.Error("Unable to load process watcher state. [%v]", err)
		return err
	}

	//initial processes checks
	module.checkForDeadProcesses()
	module.checkForNewProcesses()

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
				loop := true
				for loop {
					select {
					case <-localModule.shutdownSignal:
						loop = false
					case <-time.After(2 * time.Second):
						localModule.checkForDeadProcesses()
						localModule.checkForNewProcesses()
					}
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

func AddProcess(pid int, name string, severity string, channel string) error {
	lock.RLock()
	localModule := module
	lock.RUnlock()

	if localModule == nil {
		return errors.New("Module is not active")
	}

	var err error = nil
	if localModule.r.Acquire() {
		err = localModule.addProcessInternal(pid, name, severity, channel)
		if err == nil {
			localModule.runSaveState()
		}

		localModule.r.Release()
	}

	return err
}

func RemoveProcess(pid int, channel string) error {
	lock.RLock()
	localModule := module
	lock.RUnlock()

	if localModule == nil {
		return errors.New("Module is not active")
	}

	if localModule.r.Acquire() {
		found := false

		{
			localModule.processListMtx.Lock()

			listLen := len(localModule.processList)
			for i := listLen; i > 0; i-- {
				if localModule.processList[i-1].Pid == pid && localModule.processList[i-1].Channel == channel {
					found = true

					//from https://github.com/golang/go/wiki/SliceTricks to avoid leaks
					localModule.processList[i-1] = localModule.processList[listLen-1]
					localModule.processList[listLen-1] = nil
					localModule.processList = localModule.processList[:(listLen - 1)]
					break
				}
			}

			localModule.processListMtx.Unlock()
		}

		if found {
			localModule.runSaveState()
		}

		localModule.r.Release()
	}

	return nil
}

//------------------------------------------------------------------------------

func (m *Module) addProcessInternal(pid int, name string, severity string, channel string) error {
	var i int
	var err error = nil

	severity = settings.ValidateSeverity(severity)
	if len(severity) == 0 {
		return errors.New("Invalid type")
	}

	m.processListMtx.Lock()

	for i = len(m.processList); i > 0; i-- {
		if m.processList[i - 1].Pid == pid && m.processList[i - 1].Channel == channel {
			break
		}
	}
	if i == 0 {
		_, err = os.FindProcess(pid)
		if err == nil {
			p := &ProcessItem{
				Pid: pid,
				Name: name,
				Channel: channel,
				Severity: severity,
			}

			m.processList = append(m.processList, p)
		} else {
			err = errors.New(errProcessNotFound)
		}
	}

	m.processListMtx.Unlock()

	return err
}

func (m *Module) checkForDeadProcesses() {
	m.processListMtx.Lock()

	listLen := len(m.processList)
	for i := listLen; i > 0; i-- {
		p := m.processList[i - 1]

		_, err := os.FindProcess(p.Pid)
		if err != nil {
			//found a terminated process

			//log it
			if len(p.Name) == 0 {
				_ = logger.Log(p.Severity, p.Channel, "The process #%v has ended.", p.Pid)
			} else {
				_ = logger.Log(p.Severity, p.Channel, "The process \"%v\" (#%v) has ended.", p.Name, p.Pid)
			}

			//from https://github.com/golang/go/wiki/SliceTricks to avoid leaks
			m.processList[i - 1] = m.processList[listLen - 1]
			m.processList[listLen - 1] = nil
			m.processList = m.processList[:(listLen - 1)]
		}
	}

	m.processListMtx.Unlock()

	return
}

func (m *Module) checkForNewProcesses() {
	if len(settings.Config.Processes) > 0 {
		procs, err := gops_proc.Processes()
		if err == nil {
			//create a quick map of pids->process
			procsMap := make(map[int]*gops_proc.Process)
			for _, proc := range procs {
				procsMap[int(proc.Pid)] = proc
			}

			//verify each process
			for _, proc := range procs {
				//get the process name
				exeName, err := proc.Exe()
				if err == nil {
					//check if it matches one of the configured processes
					for _, cfgProc := range settings.Config.Processes {
						var ok bool

						if runtime.GOOS != "windows" {
							ok, _ = doublestar.PathMatch(cfgProc.ExecutableName, exeName)
						} else {
							ok, _ = doublestar.PathMatch(strings.ToLower(cfgProc.ExecutableName), strings.ToLower(exeName))
						}
						if ok {
							//we have a match
							doAdd := true

							if cfgProc.IncludeChilds {
								//check if this process is a fork of the same parent
								currentProc := proc

								//verify up to the grandparent
								for stepUp := 1; stepUp <= 2; stepUp++ {
									parentPid, err := currentProc.Ppid()
									if err != nil || parentPid == 0 {
										break
									}

									currentProc, ok = procsMap[int(parentPid)]
									if !ok {
										break
									}

									//get the parent process name and compare (case insensitive on windows)
									parentExeName, err := currentProc.Exe()
									if err == nil {
										if runtime.GOOS != "windows" {
											if parentExeName == exeName {
												doAdd = false
												break
											}
										} else {
											if strings.EqualFold(parentExeName, exeName) {
												doAdd = false
												break
											}
										}
									}
								}
							}

							if doAdd {
								//add this process to the being monitored
								name := cfgProc.FriendlyName
								if len(name) == 0 {
									name = filepath.Base(exeName)
								}

								_ = m.addProcessInternal(int(proc.Pid), name, cfgProc.Severity, cfgProc.Channel)
							}
							break
						}
					}
				}
			}
		}
	}

	return
}

func (m *Module) runSaveState() {
	if m.r.Acquire() {
		go func(m *Module) {
			m.processListMtx.Lock()
			err := m.saveState()
			m.processListMtx.Unlock()

			if err != nil {
				console.Error("Unable to save process watcher state. [%v]", err)
			}

			m.r.Release()
		}(m)
	}

	return
}
