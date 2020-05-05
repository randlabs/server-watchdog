package handlers

//------------------------------------------------------------------------------

type NotifyRequest struct {
	Channel  string `json:"channel"`
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
}

type WatchProcessRequest struct {
	Channel     string `json:"channel"`
	Pid         int `json:"pid"`
	MaxMemUsage string `json:"maxMem,omitempty"`
	Name        string `json:"name,omitempty"`
	Severity    string `json:"severity,omitempty"`
}

type UnwatchProcessRequest struct {
	Channel string `json:"channel"`
	Pid     int `json:"pid"`
}
