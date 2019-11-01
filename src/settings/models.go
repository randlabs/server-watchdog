package settings

import (
	"time"
)

//------------------------------------------------------------------------------

type SettingsJSON struct {
	Name string `json:"name,omitempty"`
	Server struct {
		Port   uint   `json:"port"`
		ApiKey string `json:"apiKey"`
	} `json:"server"`
	FileLog struct {
		Folder  string `json:"folder"`
		MaxAge  string `json:"maxAge,omitempty"`
		MaxAgeX time.Duration
	} `json:"log"`
	Channels map[string]SettingsJSON_Channel   `json:"channels"`
	Webs []SettingsJSON_Webs                   `json:"webs,omitempty"`
	FreeDiskSpace []SettingsJSON_FreeDiskSpace `json:"freeDiskSpace,omitempty"`
}

type SettingsJSON_Channel struct {
	File  *SettingsJSON_Channel_File  `json:"file,omitempty"`
	Slack *SettingsJSON_Channel_Slack  `json:"slack,omitempty"`
	EMail *SettingsJSON_Channel_EMail  `json:"email,omitempty"`
}

type SettingsJSON_Channel_File struct {
	Enabled bool   `json:"enable"`
}

type SettingsJSON_Channel_Slack struct {
	Enabled bool  `json:"enable"`
	Channel string `json:"channel"`
}

type SettingsJSON_Channel_EMail struct {
	Enabled     bool  `json:"enable"`
	Subject     string `json:"subject"`
	Sender      string `json:"sender"`
	Receivers   []string `json:"receivers"`
	Server      SettingsJSON_EMail_SmtpServer `json:"smtpServer"`
}

type SettingsJSON_EMail_SmtpServer struct {
	Host     string `json:"host"`
	Port     uint   `json:"port,omitempty"`
	UserName string `json:"username"`
	Password string `json:"password"`
	UseSSL   bool   `json:"useSSL,omitempty"`
}

type SettingsJSON_Webs struct {
	Url          string `json:"url"`
	CheckPeriod  string `json:"checkPeriod,omitempty"`
	CheckPeriodX time.Duration
	Channel      string `json:"channel"`
	Severity     string `json:"severity,omitempty"`
}

type SettingsJSON_FreeDiskSpace struct {
	Device        string `json:"device"`
	CheckPeriod   string `json:"checkPeriod,omitempty"`
	CheckPeriodX  time.Duration
	MinimumSpace  string `json:"minimumSpace"`
	MinimumSpaceX uint64
	Channel       string `json:"channel"`
	Severity      string `json:"severity,omitempty"`
}
