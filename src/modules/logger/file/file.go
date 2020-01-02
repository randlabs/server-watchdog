package file

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

type Module struct {
	shutdownSignal chan struct{}
	baseFolder     string
	maxAge         time.Duration
	newLine        string
	activeFilesMtx sync.RWMutex
	activeFiles    map[string]ActiveLogFile
	wg             sync.WaitGroup
}

type ActiveLogFile struct {
	mtx       sync.Mutex
	appName   string
	fd        *os.File
	dayOfFile string
}

//------------------------------------------------------------------------------

var newLine string
var fileModule *Module

//------------------------------------------------------------------------------

func init() {
	if runtime.GOOS == "windows" {
		newLine = "\r\n"
	} else if runtime.GOOS == "darwin " {
		newLine = "\r"
	} else {
		newLine = "\n"
	}
	return
}

func Start() error {
	//initialize module
	fileModule = &Module{}
	fileModule.shutdownSignal = make(chan struct{})

	fileModule.activeFiles = make(map[string]ActiveLogFile)

	//set up the base folder for log files
	fileModule.baseFolder = settings.Config.Log.Folder
	if len(fileModule.baseFolder) == 0 {
		fileModule.baseFolder = "logs"
	}
	if !filepath.IsAbs(fileModule.baseFolder) {
		fileModule.baseFolder = filepath.Join(settings.BaseFolder, fileModule.baseFolder)
	}
	fileModule.baseFolder = filepath.Clean(fileModule.baseFolder)
	if !strings.HasSuffix(fileModule.baseFolder, string(filepath.Separator)) {
		fileModule.baseFolder += string(filepath.Separator)
	}

	//get the maximum age for log files from settings
	fileModule.maxAge = settings.Config.Log.MaxAgeX

	//delete the old logs
	fileModule.deleteOldFiles()

	return nil
}

func Stop() {
	if fileModule != nil {
		//signal shutdown
		fileModule.shutdownSignal <- struct{}{}

		//wait until all workers are done
		fileModule.wg.Wait()

		fileModule.activeFilesMtx.Lock()

		for k := range fileModule.activeFiles {
			f := fileModule.activeFiles[k]

			f.mtx.Lock()
			if f.fd != nil {
				_ = f.fd.Sync()
				_ = f.fd.Close()
				f.fd = nil
			}
			f.mtx.Unlock()

			delete(fileModule.activeFiles, k)
		}
		fileModule.activeFilesMtx.Unlock()

		fileModule = nil
	}

	return
}

func Run(wg sync.WaitGroup) {
	if fileModule != nil {
		//start background loop
		wg.Add(1)

		go func() {
			var loop= true

			for loop {
				select {
				case <-fileModule.shutdownSignal:
					loop = false

				case <-time.After(5 * time.Minute):
					//check for old files to delete each 5 minutes
					fileModule.deleteOldFiles()
				}
			}

			wg.Done()
		}()
	}

	return
}

func Info(channel string, timestamp string, msg string) {
	fileModule.writeFileLog(channel, "[INFO]", timestamp, msg)
	return
}

func Warn(channel string, timestamp string, msg string) {
	fileModule.writeFileLog(channel, "[WARN]", timestamp, msg)
	return
}

func Error(channel string, timestamp string, msg string) {
	fileModule.writeFileLog(channel, "[ERROR]", timestamp, msg)
	return
}

//------------------------------------------------------------------------------

func (module *Module) deleteOldFiles() {
	deleteOldFilesRecursive(module.baseFolder, time.Now().Add(-module.maxAge))
	return
}

func deleteOldFilesRecursive(folder string, lowestTime time.Time) {
	files, err := ioutil.ReadDir(folder)
	if err == nil {
		for _, f := range files {
			if !f.IsDir() {
				nameLC := strings.ToLower(f.Name())
				if strings.HasSuffix(nameLC, ".log") {
					if f.ModTime().Before(lowestTime) {
						_ = os.Remove(folder + f.Name())
					}
				}
			} else {
				if f.Name() != "." && f.Name() != ".." {
					deleteOldFilesRecursive(folder + f.Name() + string(filepath.Separator), lowestTime)
				}
			}
		}
	}
	return
}

func (module *Module) getActiveLogFile(channel string) *ActiveLogFile {
	var ok bool
	var f ActiveLogFile
	var newF ActiveLogFile

	module.activeFilesMtx.RLock()
	f, ok = module.activeFiles[channel]
	module.activeFilesMtx.RUnlock()
	if ok {
		return &f
	}

	newF = ActiveLogFile{}
	newF.appName = channel
	newF.dayOfFile = ""

	module.activeFilesMtx.Lock()
	f, ok = module.activeFiles[channel]
	if ok {
		module.activeFilesMtx.Unlock()
		return &f
	}

	module.activeFiles[channel] = newF
	module.activeFilesMtx.Unlock()

	return &newF
}

func (module *Module) writeFileLog(channel string, title string, timestamp string, msg string) {
	module.wg.Add(1)

	ch, ok := settings.Config.Channels[channel]
	if !ok {
		module.wg.Done()
		return
	}
	if ch.File == nil || (!ch.File.Enabled) {
		module.wg.Done()
		return
	}

	f := module.getActiveLogFile(channel)
	if f == nil {
		module.wg.Done()
		return
	}

	go func(f *ActiveLogFile, timestamp string, msg string) {
		var err error

		err = nil

		f.mtx.Lock()
		defer f.mtx.Unlock()

		if f.fd == nil || timestamp[0:10] != f.dayOfFile {
			if f.fd != nil {
				_ = f.fd.Sync()
				_ = f.fd.Close()
				f.fd = nil
			}

			folder := module.baseFolder + f.appName + string(filepath.Separator)

			_ = os.MkdirAll(folder, 0755)

			filename := folder + f.appName + "." + timestamp[0:10] + ".log"

			f.fd, err = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
			if err == nil {
				f.dayOfFile = timestamp[0:10]
			}

		}

		if err == nil {
			_, err = f.fd.WriteString("[" + timestamp + "] " + title + " - " + msg + newLine)
		}

		if err != nil {
			console.Error("Unable to save notification in file. [%v]", err)
		}

		module.wg.Done()
	}(f, timestamp, msg)

	return
}
