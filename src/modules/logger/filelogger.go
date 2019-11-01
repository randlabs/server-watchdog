package logger

import (
	"fmt"
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

type FileLoggerModule struct {
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
	dayOfFile int
}

//------------------------------------------------------------------------------

var newLine string
var fileModule *FileLoggerModule

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

func FileLoggerStart() error {
	//initialize module
	fileModule = &FileLoggerModule{}
	fileModule.shutdownSignal = make(chan struct{})

	fileModule.activeFiles = make(map[string]ActiveLogFile)

	//set up the base folder for log files
	fileModule.baseFolder = settings.Config.FileLog.Folder
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
	fileModule.maxAge = settings.Config.FileLog.MaxAgeX

	//delete the old logs
	fileModule.deleteOldFiles()

	return nil
}

func FileLoggerStop() {
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

func FileLoggerRun(wg sync.WaitGroup) {
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

func FileLoggerInfo(channel string, format string, a ...interface{}) {
	fileModule.writeFileLog(channel, "[INFO]", format, a...)
	return
}

func FileLoggerWarn(channel string, format string, a ...interface{}) {
	fileModule.writeFileLog(channel, "[WARN]", format, a...)
	return
}

func FileLoggerError(channel string, format string, a ...interface{}) {
	fileModule.writeFileLog(channel, "[ERROR]", format, a...)
	return
}

//------------------------------------------------------------------------------

func (module *FileLoggerModule) deleteOldFiles() {
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

func (module *FileLoggerModule) getActiveLogFile(channel string) *ActiveLogFile {
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
	newF.dayOfFile = -1

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

func (module *FileLoggerModule) writeFileLog(channel string, title string, format string, a ...interface{}) {
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

	go func(f *ActiveLogFile, msg string) {
		var err error

		err = nil

		f.mtx.Lock()
		defer f.mtx.Unlock()

		now := time.Now().UTC()
		if f.fd == nil || now.Day() != f.dayOfFile {
			if f.fd != nil {
				_ = f.fd.Sync()
				_ = f.fd.Close()
				f.fd = nil
			}

			folder := module.baseFolder + f.appName + string(filepath.Separator)

			_ = os.MkdirAll(folder, 0755)

			filename := folder + f.appName + "." + now.Format("2006-01-02") + ".log"

			f.fd, err = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
			if err == nil {
				f.dayOfFile = now.Day()
			}

		}

		if err == nil {
			_, err = f.fd.WriteString("[" + now.Format("2006-01-02 15:04:05") + "] " + title + " - " + msg + newLine)
		}

		if err != nil {
			console.Error("Unable to save notification in file. [%v]", err)
		}

		module.wg.Done()
	}(f, fmt.Sprintf(format, a...))

	return
}
