# Server-Watchdog

An all-in-one server tool which accepts and delivers messages from client applications, monitors processes, web sites and free disk space.

# Quick setup

1. Download the latest binary for your operating system from the GitHub's releases page.
2. Create the configuration `settings.json` file. (See documentation below for details).
3. Run `serverwatcher --settings path-to-settings-json --service install` to configure the tool as a service.
4. Start the service.

# Usage

##### `serverwatcher --settings path-to-settings-json --service install`

Configures the server to run as a service using the specified configuration file.

E.g.: `serverwatcher --settings ./my-config.json --service install`

##### `serverwatcher --service uninstall`

Removes the service from the system.

##### `serverwatcher --service start`

Manually starts the server service.

##### `serverwatcher --service stop`

Manually stops the server service.

##### `serverwatcher --settings path-to-settings-json`

Runs the server monitoring tool as a standalone application.


# Configuration file

<details><summary>Click here to expand a sample configuration file</summary>
<p>

```json
{
	"name": "Watchdog Demo",
	"server": {
		"port": 3004,
		"apiKey": "set-some-key"
	},
	"log": {
		"folder": "./logs",
		"maxAge": "7d",
		"useLocalTime": true
	},
	"channels": {
		"default": {
			"file": {
				"enable": true
			},
			"slack": {
				"enable": true,
				"channel": "xxx/yyy/zzz"
			},
			"email": {
				"enable": true,
				"sender": "support@my-site.com",
				"subject": " Hey listen. Something happened!!",
				"receivers": [ "tech-guy@my-site.com" ],
				"smtpServer": {
					"username": "tech-guy@my-site.com",
					"password": "{super-secret-password}",
					"host": "smtp.my-site.com",
					"port":25,
					"useSSL": false
				}
			}
		},
		"channel1": {
			"file": {
				"enable": true
			},
			"slack": {
				"enable": true,
				"channel": "xxx/yyy/zzz2"
			}
		}
	},
	"webs": [
		{
			"url": "https://some-web.com/",
			"headers": {
				"Accept": "application/json"
			},
			"checkPeriod": "1min",
			"channel": "default",
			"content": [
				{
					"search": "<a\\s+class\\=\"link\"\\s+href\\='/block/(\\d+)'>(\\d+)</a>",
					"checkChanges": [ 1 ]
				}
			]
		},
		{
			"url": "https://www.google.com-invalid",
			"checkPeriod": "1min",
			"channel": "default"
		}
	],
	"processes": [
		{
			"executableName": "**\\mongod.exe",
			"includeChilds": true,
			"channel": "default"
		}
	],
	"tcpPorts": [
		{
			"name": "MongoDB",
			"address": "127.0.0.1",
			"ports": "27017",
			"channel": "default"
		}
	],
	"freeDiskSpace": [
		{
			"device": "/mnt/volume1",
			"checkPeriod": "10secs",
			"minimumSpace": "50G",
			"channel": "default"
		}
	]
}
```

</p>
</details>

#### `name`

A custom short name for this server instance. Mainly used to differentiate several servers writing to the same location.

#### `server`

Defines server parameters.

##### `server.port`

The socket port number for listening for incoming connections.

##### `server.apiKey`

A string that specifies the access token. Clients that connects to this server MUST provide the same API key. Keep this value secret.

#### `log`

Defines file logging parameters.

##### `log.folder`

Location where text log files are stored.

##### `log.maxAge`

Sets the maximum age to keep old log files. A new log file is created each day. Units can be: `s`, `sec` or `secs` for seconds; `m`, `min` or `mins` for minutes; `h`,`hour` or `hours` for hours; `d`, `day` or `days` for days and `w`, `week` or `weeks` for weeks.

##### `log.useLocalTime`

Boolean value indicating if timestamps should be in GMT or local computer time.

#### `channels`

Defines one or more channels. Channels are used by client applications and the server itself to group receivers. You can share the same channel to be used by different backends.

##### `channels.{channel-name}`

Replace `{channel-name}` with a valid object name. The `default` channel is mandatory.

##### `channels.{channel-name}.file` (optional)
    
Designates the file configuration for this channel.

##### `channels.{channel-name}.file.enabled`

If true, a log entry is added to the file log. These log files are stored under a subdirectory with the same name of the channel inside the directory specified in the `log.folder` option.

##### `channels.{channel-name}.slack` (optional)
    
Specifies the Slack webhook configuration for this channel.

##### `channels.{channel-name}.slack.enabled`

If true, messages are sent to the specified Slack channel.

##### `channels.{channel-name}.slack.channel`

Designates the target Slack WebHook channel for messaage delivery. The channel format is `T00000000/B00000000/XXXXXXXXXXX`. See
[this page](https://api.slack.com/messaging/webhooks#posting_with_webhooks) for details.

##### `channels.{channel-name}.slack.severity`

Sets the severity type of the notification: `error`, `warn`, `info` or `debug`.

##### `channels.{channel-name}.email` (optional)
    
Specifies the email delivery for this channel.

##### `channels.{channel-name}.email.enabled`

If true, messages are sent by email to the specified address(es).

##### `channels.{channel-name}.email.sender`

Indicates the sender email address.

##### `channels.{channel-name}.email.subject`

Indicates the email's subject to use.

##### `channels.{channel-name}.email.receivers`

Indicates an array of email receiver's address.

##### `channels.{channel-name}.email.smtpServer`

Specifies the SMTP server connection settings.

##### `channels.{channel-name}.email.smtpServer.username`

Defines the SMTP server access user name.

##### `channels.{channel-name}.email.smtpServer.password`

Defines the SMTP server access password.

##### `channels.{channel-name}.email.smtpServer.host`

Defines the SMTP server host name.

##### `channels.{channel-name}.email.smtpServer.port`

Defines the SMTP server port.

##### `channels.{channel-name}.email.smtpServer.useSSL`

Specifies if the connection to the SMTP server must use a secure channel.

#### `webs` (optional)

Defines an optional array of one or more web sites to be monitored. If a site is down or the content remains the same, a warning notification is sent to the configured channels.

##### `webs[].url`

Specifies the web url to monitor.

##### `webs[].headers` (optional)

Defines optionals headers to send in the request.

##### `webs[].checkPeriod`

Establishes how often the check should be done. Time units are the same than `log.maxAge`.

##### `webs[].content` (optional)

If a content is specified, besides checking if the web is online, its contents is checked. Define an array of one or more items to verify.

This is useful for live pages where contents usually depends on a backend server. If you render a page and the contents is the same, probably the backend is not working properly.

##### `webs[].content.search`

A regex string to search within the web page contents. Group check points inside parenthesis.

##### `webs[].content.checkChanges`

An array of checkpoint indexes to verify (first index is 1). A regex string can contain more than one grouping sequence. Specify only the relevants to check if the content changed or not.

##### `webs[].timeout` (optional)

Sets the maximum time to wait for the web to give a response. If not specified, a timeout of 10 seconds will be used.

##### `webs[].channel`

Establishes the channel to use when a notification must be sent because the check failed.

##### `webs[].severity`

Sets the severity type of the notification: `error`, `warn`, `info` or `debug`.

#### `tcpPorts` (optional)

Defines an optional array of one or more TCP ports to monitor.

##### `tcpPorts[].name`

Specifies the name of a group of TCP ports.

##### `tcpPorts[].address` (optional)

Specifies the host or IP address to monitor.

##### `tcpPorts[].ports` (optional)

Specifies a list of ports to verify on the address above. Use a comma separated list of port numbers. Also you can use `#-#` to define a range of ports.

##### `tcpPorts[].timeout` (optional)

Sets the maximum time to wait for the port to connect. If not specified, a timeout of 10 seconds will be used.

##### `tcpPorts[].channel`

Establishes the channel to use when a notification must be sent because the any of the tcp ports is not listening.

##### `tcpPorts[].severity`

Sets the severity type of the notification: `error`, `warn`, `info` or `debug`.

#### `processes` (optional)

Defines an optional array of one or more processes to monitor.

##### `processes[].executableName`

Specifies the process executable name. Glob wildcards (`**`, `*` and `?`) accepted.

##### `processes[].args` (optional)

Specifies optional command line arguments to include in the check. Wildcards (`*` and `?`) accepted.

##### `processes[].includeChilds` (optional)

Specifies if forks of the same process must be monitored too.

##### `processes[].channel`

Establishes the channel to use when a notification must be sent because the process abnormally exits.

##### `processes[].severity`

Sets the severity type of the notification: `error`, `warn`, `info` or `debug`.

#### `freeDiskSpace` (optional)

Defines an optional array of one or more disk devices to be monitored. If free disk space is smaller than the specified threshold, a warning notification is sent to the configured channels.

##### `freeDiskSpace[].device`

Specifies the directory to monitor. E.g.: `C:\`, `/`, `/mnt/volume1/`.

##### `freeDiskSpace[].checkPeriod`

Establishes how often the check should be done. Time units are the same than `log.maxAge`.

##### `freeDiskSpace[].minimumSpace`

The minimum required space for this disk device. Units can be `b` or `bytes` for bytes; `k`, `kb` or `kilobytes` for kilobytes; `m`, ` mb` or `megabytes` for megabytes and `g`, ` gb` or `gigabytes` for gigabytes. Floating point numbers are accepted, i.e., `1.5G`.

##### `freeDiskSpace[].channel`

Establishes the channel to use when a notification must be sent because the check failed.

##### `freeDiskSpace[].severity`

Sets the severity type of the notification: `error`, `warn`, `info` or `debug`.

# License

See [LICENSE](LICENSE) file.
