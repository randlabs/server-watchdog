package handlers

import (
	"encoding/json"
	"github.com/randlabs/server-watchdog/modules/processwatcher"
	"strings"

	"github.com/randlabs/server-watchdog/modules/logger"
	"github.com/randlabs/server-watchdog/server"
	"github.com/randlabs/server-watchdog/settings"
)

//------------------------------------------------------------------------------

func Initialize(router *server.Router) {
	router.GET("/ping", onGetPing)
	router.POST("/notify", onPostNotify)
	router.POST("/process/watch", onPostWatchProcess)
	router.POST("/process/unwatch", onPostUnwatchProcess)
	return
}

//------------------------------------------------------------------------------

func onGetPing(ctx *server.RequestCtx) {
	ctx.WriteString("pong!")
	server.SendSuccess(ctx)
	return
}

func onPostNotify(ctx *server.RequestCtx) {
	var r NotifyRequest
	var err error

	if !checkApiKey(ctx) {
		return
	}

	err = json.Unmarshal(ctx.PostBody(), &r)
	if err != nil {
		server.SendBadRequest(ctx, "")
		return
	}

	//validate channel
	if !settings.ValidateChannel(r.Channel) {
		server.SendBadRequest(ctx, "Channel not found or not specified")
		return
	}

	//validate message
	if len(r.Message) == 0 {
		server.SendBadRequest(ctx, "No message")
		return
	}

	//do log
	err = logger.Log(r.Severity, r.Channel, "%v", r.Message)
	if err != nil {
		server.SendBadRequest(ctx, err.Error())
		return
	}

	//done
	server.SendSuccess(ctx)
	return
}

func onPostWatchProcess(ctx *server.RequestCtx) {
	var r WatchProcessRequest
	var err error

	if !checkApiKey(ctx) {
		return
	}

	err = json.Unmarshal(ctx.PostBody(), &r)
	if err != nil {
		server.SendBadRequest(ctx, "")
		return
	}

	//validate channel
	if !settings.ValidateChannel(r.Channel) {
		server.SendBadRequest(ctx, "Channel not found or not specified")
		return
	}

	//validate process id
	if r.Pid < 1 || r.Pid > 0x7FFFFFFF {
		server.SendBadRequest(ctx, "Invalid process id")
		return
	}

	//add to watch list
	err = processwatcher.AddProcess(r.Pid, r.Name, r.Severity, r.Channel)
	if err != nil {
		server.SendBadRequest(ctx, err.Error())
		return
	}

	//done
	server.SendSuccess(ctx)
	return
}

func onPostUnwatchProcess(ctx *server.RequestCtx) {
	var r UnwatchProcessRequest
	var ok bool
	var err error

	if !checkApiKey(ctx) {
		return
	}

	err = json.Unmarshal(ctx.PostBody(), &r)
	if err != nil {
		server.SendBadRequest(ctx, "")
		return
	}

	//validate channel
	if len(r.Channel) == 0 {
		server.SendBadRequest(ctx, "No channel")
		return
	}
	r.Channel = strings.ToLower(r.Channel)
	_, ok = settings.Config.Channels[r.Channel]
	if !ok {
		server.SendBadRequest(ctx, "Channel not found")
		return
	}

	//validate process id
	if r.Pid < 1 || r.Pid > 0x7FFFFFFF {
		server.SendBadRequest(ctx, "Invalid process id")
		return
	}

	//remove from watch list
	processwatcher.RemoveProcess(r.Pid, r.Channel)

	//done
	server.SendSuccess(ctx)
	return
}

func checkApiKey(ctx *server.RequestCtx) bool {
	var apiKey []byte

	apiKey = ctx.Request.Header.Peek("X-Api-Key")
	if apiKey == nil {
		server.SendAccessDenied(ctx, "")
		return false
	}
	if string(apiKey) != settings.Config.Server.ApiKey {
		server.SendAccessDenied(ctx, "")
		return false
	}
	return true
}
