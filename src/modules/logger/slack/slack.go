package slack

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

type Module struct {
	shutdownSignal chan struct{}
	wg             sync.WaitGroup
}

type SlackRequestBody struct {
	Text string `json:"text"`
}

//------------------------------------------------------------------------------

var slackModule *Module

//------------------------------------------------------------------------------

func Start() error {
	//initialize module
	slackModule = &Module{}
	slackModule.shutdownSignal = make(chan struct{})

	return nil
}

func Stop() {
	if slackModule != nil {
		//signal shutdown
		slackModule.shutdownSignal <- struct{}{}

		//wait until all workers are done
		slackModule.wg.Wait()

		slackModule = nil
	}

	return
}

func Run(wg sync.WaitGroup) {
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

func Info(channel string, timestamp string, msg string) {
	slackModule.sendSlackNotification(channel, "[INFO]", timestamp, msg)
	return
}

func Warn(channel string, timestamp string, msg string) {
	slackModule.sendSlackNotification(channel, "[WARN]", timestamp, msg)
	return
}

func Error(channel string, timestamp string, msg string) {
	slackModule.sendSlackNotification(channel, "[ERROR]", timestamp, msg)
	return
}

//------------------------------------------------------------------------------

func (module *Module) sendSlackNotification(channel string, title string, timestamp string, msg string) {
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
	go func(slackChannel string, timestamp string, msg string) {
		var msgBody []byte
		var req *http.Request
		var res *http.Response
		var client *http.Client
		var resBuf *bytes.Buffer
		var err error

		_ = timestamp // avoid declared and not used

		msgBody, _ = json.Marshal(SlackRequestBody{
			Text: title + " " + settings.Config.Name + ": " + msg,
		})
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
	}(ch.Slack.Channel, timestamp, msg)
}
