/*
Golang implementation of a rundown protection for accessing a shared object

Source code and other details for the project are available at GitHub:

	https://github.com/RandLabs/rundown-protection

More usage please see README.md and tests.
*/

package directories

import (
	"path/filepath"
	"strings"
)

//------------------------------------------------------------------------------

func GetHomeDirectory() (string, error) {
	d, err := getHomeDirectory()
	if err == nil {
		if !strings.HasSuffix(d, string(filepath.Separator)) {
			d += string(filepath.Separator)
		}
	}
	return d, err
}

func GetAppSettingsDirectory() (string, error) {
	d, err := getAppSettingsDirectory()
	if err == nil {
		if !strings.HasSuffix(d, string(filepath.Separator)) {
			d += string(filepath.Separator)
		}
	}
	return d, err
}

func GetSystemSettingsDirectory() (string, error) {
	d, err := getSystemSettingsDirectory()
	if err == nil {
		if !strings.HasSuffix(d, string(filepath.Separator)) {
			d += string(filepath.Separator)
		}
	}
	return d, err
}
