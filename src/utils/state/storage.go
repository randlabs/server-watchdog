package state

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/randlabs/directories"
)

//------------------------------------------------------------------------------

const (
	maxStateFileSize = 100 * 1048576
)

//------------------------------------------------------------------------------

var createDirOnce struct {
	m    sync.Mutex
	done uint32
	dir  string
}

//------------------------------------------------------------------------------

func LoadStateBlob(filename string) ([]byte, error) {
	var b []byte

	dir, err := getConfigDir()
	if err == nil {
		f, err := os.OpenFile(dir + filename, os.O_RDONLY, 0644)
		if err == nil {
			defer func() {
				_ = f.Close()
			}()

			fi, err := f.Stat()
			if err == nil && fi.Size() > 0 {
				if fi.Size() <= maxStateFileSize {
					b = make([]byte, fi.Size())
					_, err = f.Read(b, )
				} else {
					err = errors.New("invalid file")
				}
			}
		} else {
			if os.IsNotExist(err) {
				err = nil
			}
		}
	}
	return b, err
}

func SaveStateBlob(filename string, blob []byte) error {
	dir, err := getConfigDir()
	if err == nil {
		f, err := os.OpenFile(dir + filename, os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer func() {
				_ = f.Close()
			}()

			_, err = f.Write(blob)
		}
	}
	if err != nil {
		_ = os.Remove(dir + filename)
	}
	return err
}

func DeleteStateBlob(filename string) {
	dir, err := getConfigDir()
	if err == nil {
		_ = os.Remove(dir + filename)
	}
	return
}

//------------------------------------------------------------------------------

func getConfigDir() (string, error) {
	if atomic.LoadUint32(&createDirOnce.done) == 0 {
		createDirOnce.m.Lock()
		defer createDirOnce.m.Unlock()

		if createDirOnce.done == 0 {
			dir, err := directories.GetAppSettingsDirectory()
			if err != nil {
				return "", err
			}
			dir += "randlabs" + string(filepath.Separator) + "server-watchdog" + string(filepath.Separator)

			err = os.MkdirAll(dir, 0755)
			if err != nil {
				return "", err
			}
			createDirOnce.dir = dir

			atomic.StoreUint32(&createDirOnce.done, 1)
		}
	}
	return createDirOnce.dir, nil
}
