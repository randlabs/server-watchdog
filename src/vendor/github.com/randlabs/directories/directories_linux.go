package directories

import (
	"fmt"
	"os/user"
	"strings"
)

//------------------------------------------------------------------------------

// reference https://refspecs.linuxfoundation.org/FHS_3.0/fhs/ch03s07.html

func getHomeDirectory() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("unable to retrieve home directory")
	}
	return usr.HomeDir, nil
}

func getAppSettingsDirectory() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("unable to retrieve application settings path")
	}
	d := usr.HomeDir
	if !strings.HasSuffix(d, "/") {
		d += "/"
	}
	d += ".config"
	return d, nil
}

func getSystemSettingsDirectory() (string, error) {
	return "/etc", nil
}
