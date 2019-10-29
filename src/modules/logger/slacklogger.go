package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

type SlackLoggerModule struct {
	shutdownSignal chan bool
	baseFolder string
	maxAge time.Duration
	newLine string
	activeFilesMtx sync.RWMutex
	activeFiles map[string]ActiveLogFile
	wg sync.WaitGroup
}

type SlackRequestBody struct {
	Text string `json:"text"`
}

//------------------------------------------------------------------------------

var slackModule *SlackLoggerModule

//------------------------------------------------------------------------------

func SlackLoggerStart() error {
	//initialize module
	slackModule = &SlackLoggerModule{}
	slackModule.shutdownSignal = make(chan bool, 1)

	return nil
}

func SlackLoggerStop() {
	if slackModule != nil {
		//signal shutdown
		slackModule.shutdownSignal <- true

		//wait until all workers are done
		slackModule.wg.Wait()

		slackModule = nil
	}

	return
}

func SlackLoggerRun(wg sync.WaitGroup) {
	if slackModule != nil {
		//start background loop
		wg.Add(1)

		go func() {
			//just wait for the shutdown signal
			<-slackModule.shutdownSignal

			wg.Done()
		}()
	}

	return
}

func SlackLoggerInfo(channel string, format string, a ...interface{}) {
	//no op
	return
}

func SlackLoggerWarn(channel string, format string, a ...interface{}) {
	slackModule.sendSlackNotification(channel, "[WARN]", format, a...)
	return
}

func SlackLoggerError(channel string, format string, a ...interface{}) {
	slackModule.sendSlackNotification(channel, "[ERROR]", format, a...)
	return
}

//------------------------------------------------------------------------------

func (module *SlackLoggerModule) sendSlackNotification(channel string, title string, format string, a ...interface{}) {
	module.wg.Add(1)

	//retrieve channel info and check if enabled
	ch, ok := settings.Config.Channels[channel]
	if !ok {
		module.wg.Done()
		return
	}
	if ch.Slack == nil || (!ch.Slack.Enabled) {
		module.wg.Done()
		return
	}

	//do notification
	go func(slackChannel string, msg string) {
		var msgBody []byte
		var req *http.Request
		var res *http.Response
		var client *http.Client
		var resBuf *bytes.Buffer
		var err error

		msgBody, _ = json.Marshal(SlackRequestBody{Text: title + " " + msg})
		req, err = http.NewRequest(http.MethodPost, "https://hooks.slack.com/services/" + slackChannel,
						bytes.NewBuffer(msgBody))
		if err == nil {
			req.Header.Add("Content-Type", "application/json")

			client = &http.Client{Timeout: 10 * time.Second}
			res, err = client.Do(req)
			if err == nil {
				resBuf = new(bytes.Buffer)
				_, err = resBuf.ReadFrom(res.Body)
				if err == nil {
					if resBuf.String() != "ok" {
						err = errors.New("Unsuccessful response returned from Slack")
					}
				}
			}
		}

		if err != nil {
			console.Error("Unable to deliver notification to Slack channel. [%v]", err)
		}

		module.wg.Done()
	}(ch.Slack.Channel, fmt.Sprintf(format, a...))
}
