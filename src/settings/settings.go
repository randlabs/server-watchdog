package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/RoaringBitmap/roaring"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	valid "github.com/asaskevich/govalidator"
	"github.com/randlabs/server-watchdog/utils/process"
	"github.com/randlabs/server-watchdog/utils/stringparser"
	"github.com/ricochet2200/go-disk-usage/du"
)

//------------------------------------------------------------------------------

var Config SettingsJSON
var BaseFolder string

//------------------------------------------------------------------------------

// Load ...
func Load() error {
	var file *os.File
	var parser *json.Decoder
	var settingsFilename string
	var ok bool
	var err error

	settingsFilename, err = GetSettingsFilename()
	if err != nil {
		return err
	}

	file, err = os.Open(settingsFilename)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot load settings. [%v]", err))
	}
	defer func() {
		_ = file.Close()
	}()

	BaseFolder = filepath.Dir(settingsFilename)
	if !strings.HasSuffix(BaseFolder, string(filepath.Separator)) {
		BaseFolder += string(filepath.Separator)
	}

	parser = json.NewDecoder(file)
	err = parser.Decode(&Config)
	if err != nil {
		return errors.New(fmt.Sprintf("Invalid settings file. [%v]", err))
	}

	//validate settings
	if len(Config.Name) == 0 {
		Config.Name = "SERVER-WATCHDOG"
	} else if len(Config.Name) > 256 {
		return errors.New("Name is too long. Max 256 chars.")
	}

	//----

	if Config.Server.Port < 1 || Config.Server.Port > 65535 {
		return errors.New(fmt.Sprintf("Invalid server port."))
	}
	if len(Config.Server.ApiKey) == 0 {
		return errors.New(fmt.Sprintf("Invalid server API key."))
	}

	//----

	if len(Config.Log.MaxAge) > 0 {
		Config.Log.MaxAgeX, ok = ValidateTimeSpan(Config.Log.MaxAge)
		if !ok {
			return errors.New("Invalid log files max age value.")
		}
		if Config.Log.MaxAgeX < 10 * time.Minute {
			return errors.New("Log files max age value cannot be lower than 10 minutes.")
		}
	} else {
		Config.Log.MaxAgeX = 7 * 24 * time.Hour
	}

	//----

	hasChannels := false
	for chName := range Config.Channels {
		hasChannels = true

		hasOutput := false

		if len(chName) == 0 {
			return errors.New(fmt.Sprintf("A channel without name was specified."))
		} else if len(chName) == 32 {
			return errors.New(fmt.Sprintf("Channel name too long. Max 32 chars."))
		}

		ch := Config.Channels[chName]

		if ch.File != nil && ch.File.Enabled {
			hasOutput = true
		}

		if ch.Slack != nil && ch.Slack.Enabled {
			hasOutput = true
			if len(ch.Slack.Channel) == 0 {
				return errors.New(fmt.Sprintf("No slack hook specified for channel \"%v\".", chName))
			}
		}

		if ch.EMail != nil && ch.EMail.Enabled {
			hasOutput = true

			if len(ch.EMail.Subject) > 256 {
				return errors.New(fmt.Sprintf("Email subject for channel \"%v\" is too long. Max 256 chars.", chName))
			}

			if !valid.IsEmail(ch.EMail.Sender) {
				return errors.New(fmt.Sprintf("Missing or invalid sender email address for channel \"%v\".", chName))
			}

			if ch.EMail.Receivers == nil || len(ch.EMail.Receivers) == 0 {
				return errors.New(fmt.Sprintf("No receiver email addresses for channel \"%v\" was specified.", chName))
			}

			for i := len(ch.EMail.Receivers); i > 0; i-- {
				if !valid.IsEmail(ch.EMail.Receivers[i - 1]) {
					return errors.New(fmt.Sprintf("Invalid receiver email address specified for channel \"%v\".", chName))
				}
			}

			if len(ch.EMail.Server.UserName) == 0 {
				return errors.New(fmt.Sprintf("Missing email server's username for channel \"%v\".", chName))
			}
			if len(ch.EMail.Server.Host) == 0 {
				return errors.New(fmt.Sprintf("Missing email server's host for channel \"%v\".", chName))
			}

			if ch.EMail.Server.Port == 0 {
				ch.EMail.Server.Port = 25
			} else if ch.EMail.Server.Port < 1 || ch.EMail.Server.Port > 65535 {
				return errors.New(fmt.Sprintf("Invalid email server's port for channel \"%v\".", chName))
			}
		}

		if !hasOutput {
			return errors.New(fmt.Sprintf("No output stream was specified for channel \"%v\".", chName))
		}
	}
	if !hasChannels {
		return errors.New(fmt.Sprintf("No channels were specified."))
	}

	//----

	for idx := range Config.Processes {
		proc := &Config.Processes[idx]

		if len(proc.ExecutableName) == 0 {
			return errors.New(fmt.Sprintf("Missing or invalid process' executable name."))
		}

		_, ok = Config.Channels[proc.Channel]
		if !ok {
			return errors.New(fmt.Sprintf("Channel not found for process \"%v\".", proc.ExecutableName))
		}

		proc.Severity = ValidateSeverity(proc.Severity)
		if len(proc.Severity) == 0 {
			return errors.New(fmt.Sprintf("Invalid severity for process \"%v\".", proc.ExecutableName))
		}
	}

	//----

	for idx := range Config.Webs {
		web := &Config.Webs[idx]

		if !valid.IsURL(web.Url) {
			return errors.New(fmt.Sprintf("Missing or invalid url specified."))
		}

		if len(web.CheckPeriod) > 0 {
			web.CheckPeriodX, ok = ValidateTimeSpan(web.CheckPeriod)
			if !ok {
				return errors.New(fmt.Sprintf("Invalid web check period value for web \"%v\".", web.Url))
			}
			if web.CheckPeriodX < 10 * time.Second {
				return errors.New(fmt.Sprintf("Web check period value for \"%v\" cannot be lower than 10 seconds.", web.Url))
			}
		} else {
			web.CheckPeriodX = 10 * time.Second
		}

		for contentIdx := range web.Content {
			wc := &web.Content[contentIdx]

			if len(wc.Search) == 0 {
				return errors.New(fmt.Sprintf("Missing content search regex for web \"%v\".", web.Url))
			}
			wc.SearchRegex, err = regexp.Compile(wc.Search)
			if err != nil {
				return errors.New(fmt.Sprintf("Invalid content search regex for web \"%v\".", web.Url))
			}

			nSubExpr := uint(wc.SearchRegex.NumSubexp())
			for idx := range wc.CheckChanges {
				if wc.CheckChanges[idx] < 1 || wc.CheckChanges[idx] > nSubExpr {
					return errors.New(fmt.Sprintf("Invalid content search regex for web \"%v\".", web.Url))
				}
			}
		}

		if len(web.Timeout) > 0 {
			web.TimeoutX, ok = ValidateTimeSpan(web.Timeout)
			if !ok {
				return errors.New(fmt.Sprintf("Invalid web check timeout value for web \"%v\".", web.Url))
			}
			if web.TimeoutX < 10 * time.Second {
				return errors.New(fmt.Sprintf("Web check timeout value for \"%v\" cannot be lower than 10 seconds.", web.Url))
			}
		} else {
			web.TimeoutX = 10 * time.Second
		}

		_, ok = Config.Channels[web.Channel]
		if !ok {
			return errors.New(fmt.Sprintf("Channel not found for web \"%v\".", web.Url))
		}

		web.Severity = ValidateSeverity(web.Severity)
		if len(web.Severity) == 0 {
			return errors.New(fmt.Sprintf("Invalid severity for web \"%v\".", web.Url))
		}
	}

	//----

	for idx := range Config.TcpPorts {
		port := &Config.TcpPorts[idx]

		if len(port.Name) == 0 {
			return errors.New(fmt.Sprintf("Missing or invalid TCP port description name."))
		}

		if !valid.IsHost(port.Address) {
			return errors.New(fmt.Sprintf("Missing or invalid address in TCP port group \"%v\".", port.Name))
		}

		port.PortsX, ok = parsePortsList(port.Ports)
		if !ok {
			return errors.New(fmt.Sprintf("Missing or invalid port value/range in TCP port group \"%v\".", port.Name))
		}

		if len(port.CheckPeriod) > 0 {
			port.CheckPeriodX, ok = ValidateTimeSpan(port.CheckPeriod)
			if !ok {
				return errors.New(fmt.Sprintf("Invalid check period value for TCP port group \"%v\".", port.Name))
			}
			if port.CheckPeriodX < 10 * time.Second {
				return errors.New(fmt.Sprintf("Check period value for TCP port group \"%v\" cannot be lower than 10 seconds.", port.Name))
			}
		} else {
			port.CheckPeriodX = 10 * time.Second
		}

		if len(port.Timeout) > 0 {
			port.TimeoutX, ok = ValidateTimeSpan(port.Timeout)
			if !ok {
				return errors.New(fmt.Sprintf("Invalid check timeout value for TCP port group \"%v\".", port.Name))
			}
			if port.TimeoutX < 10 * time.Second {
				return errors.New(fmt.Sprintf("Check timeout value for TCP port group \"%v\" cannot be lower than 10 seconds.", port.Name))
			}
		} else {
			port.TimeoutX = 10 * time.Second
		}

		_, ok = Config.Channels[port.Channel]
		if !ok {
			return errors.New(fmt.Sprintf("Channel not found for TCP Port \"%v\".", port.Name))
		}

		port.Severity = ValidateSeverity(port.Severity)
		if len(port.Severity) == 0 {
			return errors.New(fmt.Sprintf("Invalid severity for TCP Port \"%v\".", port.Name))
		}
	}

	//----

	for idx := range Config.FreeDiskSpace {
		fds := &Config.FreeDiskSpace[idx]

		if len(fds.Device) == 0 {
			return errors.New(fmt.Sprintf("Missing or invalid url specified."))
		}

		fds.Device = filepath.Clean(fds.Device)
		if runtime.GOOS == "windows" {
			if !strings.HasSuffix(fds.Device, string(filepath.Separator)) {
				fds.Device += string(filepath.Separator)
			}
		}

		diskUsage := du.NewDiskUsage(fds.Device)
		if diskUsage.Size() == 0 {
			return errors.New(fmt.Sprintf("Invalid or missing free disk space check device \"%v\".", fds.Device))
		}
		diskSize := diskUsage.Size()

		fds.MinimumSpaceX, ok = ValidateMemoryAmount(fds.MinimumSpace, &diskSize)
		if !ok {
			return errors.New(fmt.Sprintf("Invalid free disk space check minimum value for device \"%v\".",
											fds.Device))
		}

		if len(fds.CheckPeriod) > 0 {
			fds.CheckPeriodX, ok = ValidateTimeSpan(fds.CheckPeriod)
			if !ok {
				return errors.New(fmt.Sprintf("Invalid free disk space check period value for device \"%v\".",
												fds.Device))
			}
		} else {
			fds.CheckPeriodX = -1
		}

		_, ok = Config.Channels[fds.Channel]
		if !ok {
			return errors.New(fmt.Sprintf("Channel not found for device \"%v\".", fds.Device))
		}

		fds.Severity = ValidateSeverity(fds.Severity)
		if len(fds.Severity) == 0 {
			return errors.New(fmt.Sprintf("Invalid severity for device \"%v\".", fds.Device))
		}
	}

	return nil
}

func GetSettingsFilename() (string, error) {
	filename, err := process.GetCmdLineParam("settings")
	if err != nil {
		return "", err
	}
	if len(filename) == 0 {
		filename = "./settings.json"
	}

	if !filepath.IsAbs(filename) {
		filename = filepath.Join(process.AppPath, filename)
	}
	filename = filepath.Clean(filename)
	return filename, nil
}

func ValidateSeverity(severity string) string {
	switch severity {
	case "error":
		fallthrough
	case "warn":
		fallthrough
	case "info":
		fallthrough
	case "debug":
		return severity
	case "warning":
		return "warn"
	case "information":
		return "info"
	case "":
		return "error"
	}
	return ""
}

func ValidateChannel(channel string) bool {
	_, ok := Config.Channels[channel]
	return ok
}

func ValidateMaxMemoryUsage(channel string) bool {
	_, ok := Config.Channels[channel]
	return ok
}

func ValidateTimeSpan(t string) (time.Duration, bool) {
	var d time.Duration

	width := stringparser.SkipSpaces(t)
	if width < 0 {
		return 0, false
	}
	if width > len(t) {
		return 0, false //do not accept empty strings
	}

	for width < len(t) {
		//get value
		value, w := stringparser.GetUint64(t[width:])
		if w <= 0 {
			return 0, false
		}
		width += w

		w = stringparser.SkipSpaces(t[width:])
		if w < 0 {
			return 0, false
		}
		width += w

		//get units
		units, w := stringparser.GetText(t[width:])
		if w <= 0 {
			return 0, false
		}
		width += w

		//increment d
		prevD := d
		switch strings.ToLower(units) {
		case "ms":
			d += time.Duration(value) * time.Millisecond

		case "s":
			fallthrough
		case "sec":
			fallthrough
		case "secs":
			d += time.Duration(value) * time.Second

		case "m":
			fallthrough
		case "min":
			fallthrough
		case "mins":
			d += time.Duration(value) * time.Minute

		case "h":
			fallthrough
		case "hour":
			fallthrough
		case "hours":
			d += time.Duration(value) * time.Hour

		case "d":
			fallthrough
		case "day":
			fallthrough
		case "days":
			d += time.Duration(value) * time.Hour * 24

		case "w":
			fallthrough
		case "week":
			fallthrough
		case "weeks":
			d += time.Duration(value) * time.Hour * 24 * 7

		default:
			return 0, false
		}
		if d < prevD {
			return 0, false //rollover
		}

		w = stringparser.SkipSpaces(t[width:])
		if w < 0 {
			return 0, false
		}
		width += w
	}

	return d, true
}

func ValidateMemoryAmount(t string, totalAvailable *uint64) (uint64, bool) {
	var siz uint64
	var w int

	width := stringparser.SkipSpaces(t)
	if width < 0 {
		return 0, false
	}
	if width > len(t) {
		return 0, false //do not accept empty strings
	}

	//get value
	value, w := stringparser.GetFloat64(t[width:])
	if w <= 0 {
		return 0, false
	}
	width += w

	w = stringparser.SkipSpaces(t[width:])
	if w < 0 {
		return 0, false
	}
	width += w

	//get units
	units, w := stringparser.GetText(t[width:])
	if w <= 0 {
		return 0, false
	}
	width += w

	w = stringparser.SkipSpaces(t[width:])
	if w < 0 {
		return 0, false
	}
	width += w
	if width < len(t) {
		return 0, false //not the end of the string
	}

	switch strings.ToLower(units) {
	case "":
		fallthrough
	case "b":
		fallthrough
	case "bytes":
		siz = uint64(value)

	case "k":
		fallthrough
	case "kb":
		fallthrough
	case "kilobytes":
		if value * 1024.0 < value {
			return 0, false
		}
		siz = uint64(value * 1024.0)

	case "m":
		fallthrough
	case "mb":
		fallthrough
	case "megabytes":
		if value * 1048576.0 < value {
			return 0, false
		}
		siz = uint64(value * 1048576.0)

	case "g":
		fallthrough
	case "gb":
		fallthrough
	case "gigabytes":
		if value * 1073741824.0 < value {
			return 0, false
		}
		siz = uint64(value * 1073741824.0)

	case "%":
		if totalAvailable == nil {
			return 0, false
		}
		if value < 0 || value > 100.0 {
			return 0, false
		}
		siz = uint64(value / 100.0 * float64(*totalAvailable))

	default:
		return 0, false
	}

	return siz, true
}

//------------------------------------------------------------------------------

func parsePortsList(p string) (*roaring.Bitmap, bool) {
	var s string
	var port int
	var portEnd int
	var err error

	rb := roaring.New()

	portGroups := strings.Split(p, ",")
	for i := range portGroups {
		portRange := strings.Split(portGroups[i], "-")

		switch len(portRange) {
		case 1:
			s = strings.Trim(portRange[0], " ")
			if len(s) == 0 {
				return nil, false
			}
			port, err = strconv.Atoi(s)
			if err != nil || port < 0 || port > 65535 {
				return nil, false
			}

			rb.Add(uint32(port))

		case 2:
			s = strings.Trim(portRange[0], " ")
			if len(s) == 0 {
				return nil, false
			}
			port, err = strconv.Atoi(s)
			if err != nil || port < 0 || port > 65535 {
				return nil, false
			}

			s = strings.Trim(portRange[0], " ")
			if len(s) == 0 {
				return nil, false
			}
			portEnd, err = strconv.Atoi(s)
			if err != nil || portEnd < port || portEnd > 65535 {
				return nil, false
			}

			rb.AddRange(uint64(port), uint64(portEnd) + 1)

		default:
			return nil, false
		}
	}

	if rb.IsEmpty() {
		return nil, false
	}

	return rb, true
}