package email

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"net/smtp"
	"strconv"
	"sync"

	"github.com/randlabs/server-watchdog/console"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

type Module struct {
	shutdownSignal chan struct{}
	wg             sync.WaitGroup
}

//------------------------------------------------------------------------------

var emailModule *Module

//------------------------------------------------------------------------------

func Start() error {
	//initialize module
	emailModule = &Module{}
	emailModule.shutdownSignal = make(chan struct{})

	return nil
}

func Stop() {
	if emailModule != nil {
		//signal shutdown
		emailModule.shutdownSignal <- struct{}{}

		//wait until all workers are done
		emailModule.wg.Wait()

		emailModule = nil
	}

	return
}

func Run(wg sync.WaitGroup) {
	if emailModule != nil {
		//start background loop
		wg.Add(1)

		go func() {
			//just wait for the shutdown signal
			<-emailModule.shutdownSignal

			wg.Done()
		}()
	}

	return
}

func Info(channel string, format string, a ...interface{}) {
	emailModule.sendEmailNotification(channel, "[INFO]", format, a...)
	return
}

func Warn(channel string, format string, a ...interface{}) {
	emailModule.sendEmailNotification(channel, "[WARN]", format, a...)
	return
}

func Error(channel string, format string, a ...interface{}) {
	emailModule.sendEmailNotification(channel, "[ERROR]", format, a...)
	return
}

//------------------------------------------------------------------------------

func (module *Module) sendEmailNotification(channel string, title string, format string, a ...interface{}) {
	module.wg.Add(1)

	//retrieve channel info and check if enabled
	ch, ok := settings.Config.Channels[channel]
	if !ok {
		module.wg.Done()
		return
	}
	if ch.EMail == nil || (!ch.EMail.Enabled) {
		module.wg.Done()
		return
	}

	//do notification
	go func(email *settings.SettingsJSON_Channel_EMail, channel string, title string, msg string) {
		var c *smtp.Client
		var subject string
		var err error

		servername := email.Server.Host + ":" + strconv.FormatUint(uint64(email.Server.Port), 10)

		auth := smtp.PlainAuth("", email.Server.UserName, email.Server.Password, email.Server.Host)

		from := mail.Address{ Name: "", Address: email.Sender }

		if len(ch.EMail.Subject) == 0 {
			subject = title + " " + settings.Config.Name + ": Message from channel " + channel
		} else {
			subject = title + " " + email.Subject
		}

		header := make(map[string]string)
		header["From"] = from.String()
		header["To"] = "undisclosed-recipients"
		header["Subject"] = encodeRFC2047(subject)
		header["MIME-Version"] = "1.0"
		header["Content-Type"] = "text/plain; charset=\"utf-8\""
		header["Content-Transfer-Encoding"] = "base64"

		message := ""
		for k, v := range header {
			message += fmt.Sprintf("%s: %s\r\n", k, v)
		}
		message += "\r\n" + base64.StdEncoding.EncodeToString([]byte(msg))

		if email.Server.UseSSL {
			var conn *tls.Conn

			tlsconfig := &tls.Config {
				InsecureSkipVerify: true,
				ServerName: email.Server.Host,
			}

			conn, err = tls.Dial("tcp", servername, tlsconfig)
			if err == nil {
				c, err = smtp.NewClient(conn, email.Server.Host)
			}
		} else {
			c, err = smtp.Dial(email.Server.Host)
		}

		if err == nil {
			err = c.Auth(auth)

			if err == nil {
				err = c.Mail(from.Address)
			}

			if err == nil {
				for _, rcpt := range email.Receivers {
					err = c.Rcpt(rcpt)
					if err != nil {
						break
					}
				}
			}

			if err == nil {
				var w io.WriteCloser

				w, err = c.Data()
				if err == nil {
					_, err = w.Write([]byte(message))
				}
				if err == nil {
					err = w.Close()
				}
			}

			_ = c.Quit()
		}
		if err != nil {
			console.Error("Unable to deliver notification to EMail channel. [%v]", err)
		}

		module.wg.Done()
	}(ch.EMail, channel, title, fmt.Sprintf(format, a...))
}

func encodeRFC2047(s string) string{
	return mime.QEncoding.Encode("utf-8", s)
}
