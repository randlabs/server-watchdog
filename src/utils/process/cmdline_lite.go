package process

import (
	"errors"
	"os"
)

//------------------------------------------------------------------------------

func GetCmdLineParam(key string) (string, error) {
	key = "--" + key
	for idx, arg := range os.Args {
		if arg == key {
			if idx >= len(os.Args) {
				return "", errors.New("Missing argument for " + key + " parameter.")
			}
			return os.Args[idx + 1], nil
		}
	}
	return "", nil
}
