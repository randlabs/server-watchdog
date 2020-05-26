package slack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
		close(slackModule.shutdownSignal)

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

func Error(channel string, timestamp string, msg string) {
	slackModule.sendSlackNotification(channel, "[ERROR]", timestamp, msg)
	return
}

func Warn(channel string, timestamp string, msg string) {
	slackModule.sendSlackNotification(channel, "[WARN]", timestamp, msg)
	return
}

func Info(channel string, timestamp string, msg string) {
	slackModule.sendSlackNotification(channel, "[INFO]", timestamp, msg)
	return
}

func Debug(channel string, timestamp string, msg string) {
	slackModule.sendSlackNotification(channel, "[DEBUG", timestamp, msg)
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
	go func(slackChannel string, channel string, timestamp string, msg string) {
		var msgBody []byte
		var req *http.Request
		var resp *http.Response
		var client *http.Client
		var err error

		_ = timestamp // avoid declared and not used

		msgBody, _ = json.Marshal(SlackRequestBody{
			Text: title + " " + settings.Config.Name + " / " + channel + ": " + msg,
		})

TryAgain:
		client = &http.Client{
			Timeout: 10 * time.Second,
		}

		req, err = http.NewRequest(http.MethodPost, "https://hooks.slack.com/services/" + slackChannel,
						bytes.NewBuffer(msgBody))
		if err == nil {
			req.Header.Add("Content-Type", "application/json")

			resp, err = client.Do(req)
			if err == nil {
				if resp.StatusCode == http.StatusOK {
					/*
					bodyBytes, err := ioutil.ReadAll(resp.Body)
					if err == nil {
						bodyString := string(bodyBytes)
						if bodyString != "ok" {
							err = errors.New("Unsuccessful response returned from Slack")
						}
					}
					*/
				} else if resp.StatusCode == http.StatusTooManyRequests {
					var secs int64 = 5 //default timeout

					s := resp.Header.Get("Retry-After")
					s = strings.TrimSpace(s)
					if len(s) > 0 {
						if s[0] >= '0' && s[0] <= '9' {
							deltaSecs, err := strconv.Atoi(s)
							if err == nil && deltaSecs > 0 {
								secs = int64(deltaSecs)
							}
						} else {
							timestamp, err := time.Parse(time.RFC1123, s)
							if err != nil {
								timestamp, err = time.Parse(time.RFC1123Z, s)
							}
							if err == nil {
								timestamp = timestamp.UTC()

								now := time.Now().UTC()
								if timestamp.After(now) {
									deltaSecs2 := int64(timestamp.Sub(now))
									if deltaSecs2 > 0 {
										secs = deltaSecs2
									}
								}
							}
						}
					}

					select {
					case <-module.shutdownSignal:
						err = errors.New("Canceled delivery due to shutdown")
					case <-time.After(time.Duration(secs) * time.Second):
						goto TryAgain
					}
				} else {
					err = errors.New(fmt.Sprintf("Unexpected response [status: %v]", resp.StatusCode))
				}
			}
		}

		if err != nil {
			console.Error("Unable to deliver notification to Slack channel. [%v]", err)
		}

		module.wg.Done()
	}(ch.Slack.Channel, channel, timestamp, msg)
}
