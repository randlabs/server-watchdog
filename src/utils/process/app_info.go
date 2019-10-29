package process

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

//------------------------------------------------------------------------------

//AppPath ...
var AppPath string

//AppName ...
var AppName string

//------------------------------------------------------------------------------

func init() {
	var exeName string
	var ext string
	var err error

	exeName, err = os.Executable()
	if err != nil {
		log.Fatalf("Error: Unable to get application path. [%v]\n", err)
	}

	AppPath = filepath.Clean(filepath.Dir(exeName))
	if !strings.HasSuffix(AppPath, string(filepath.Separator)) {
		AppPath += string(filepath.Separator)
	}

	exeName = filepath.Base(exeName)

	ext = filepath.Ext(exeName)
	AppName = exeName[0 : len(exeName)-len(ext)]
	return
}
