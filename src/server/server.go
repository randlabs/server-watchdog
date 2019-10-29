package server

import (
	"encoding/json"
	"errors"
	"net"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/buaazp/fasthttprouter"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
)

//------------------------------------------------------------------------------

// RequestCtx ...
type RequestCtx = fasthttp.RequestCtx

// Router ...
type Router = fasthttprouter.Router

// Cookie ...
type Cookie = fasthttp.Cookie

//------------------------------------------------------------------------------

type Server struct {
	Router *fasthttprouter.Router
	shutdownSignal chan bool
	shutdownErr chan error
	shuttingDown uint32
}

//------------------------------------------------------------------------------

var strContentType = []byte("Content-Type")
var strApplicationJSON = []byte("application/json")

//------------------------------------------------------------------------------

func Create(port uint, compress bool) (*Server, error) {
	var server *Server
	var fastserver fasthttp.Server
	var gracefulListener net.Listener
	var listener net.Listener
	var listenErr chan error
	var err error

	if port < 1 || port > 65535 {
		return nil, errors.New("Invalid server port")
	}

	server = &Server{}
	server.Router = fasthttprouter.New()
	server.shutdownSignal = make(chan bool, 1)
	server.shutdownErr = make(chan error, 1)

	// compression
	h := server.Router.Handler
	if compress {
		h = fasthttp.CompressHandler(h)
	}

	fastserver = fasthttp.Server{
		Handler:              h,
		ReadTimeout:          10 * time.Second,
		WriteTimeout:         10 * time.Second,
		MaxConnsPerIP:        20000,
		MaxRequestsPerConn:   5,
		DisableKeepalive:     true,
	}

	// create listener
	if runtime.GOOS == "windows" {
		listener, err = net.Listen("tcp4", "0.0.0.0:" + strconv.Itoa(int(port)))
	} else {
		listener, err = reuseport.Listen("tcp4", "0.0.0.0:" + strconv.Itoa(int(port)))
	}
	if err != nil {
		return nil, err
	}

	// create a graceful shutdown listener
	gracefulListener = NewGracefulListener(listener, 5 * time.Second)

	// error handling
	listenErr = make(chan error, 1)

	/// Run server
	go func() {
		listenErr <- fastserver.Serve(gracefulListener)
	}()

	go func() {
		for {
			select {
			// If server.ListenAndServe() cannot start due to errors such
			// as "port in use" it will return an error.
			case err := <-listenErr:
				if atomic.CompareAndSwapUint32(&server.shuttingDown, 0, 1) {
					server.shutdownErr <- err
				}

			// handle termination signal
			case <-server.shutdownSignal:
				// Servers in the process of shutting down should disable KeepAlives
				// FIXME: This causes a data race
				//server.SetKeepAlivesEnabled(false)

				// Attempt the graceful shutdown by closing the listener
				// and completing all inflight requests.

				_ = gracefulListener.Close()

				server.shutdownErr <- nil
			}
		}
	}()

	return server, nil
}

func (server* Server) Wait() error {
	return <-server.shutdownErr
}

func (server* Server) Stop() {
	if atomic.CompareAndSwapUint32(&server.shuttingDown, 0, 1) {
		server.shutdownSignal <- true
	}
}

func SendSuccess(ctx* RequestCtx) {
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	return
}

func SendJSON(ctx* RequestCtx, obj interface{}) {
	var err error

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(fasthttp.StatusOK)

	err = json.NewEncoder(ctx).Encode(obj)
	if  err != nil {
		SendInternalServerError(ctx, "")
	}
	return
}

func SendBadRequest(ctx *RequestCtx, msg string) {
	sendError(ctx, fasthttp.StatusBadRequest, msg)
	return
}

func SendAccessDenied(ctx *RequestCtx, msg string) {
	sendError(ctx, fasthttp.StatusForbidden, msg)
	return
}

func SendInternalServerError(ctx *RequestCtx, msg string) {
	sendError(ctx, fasthttp.StatusInternalServerError, msg)
	return
}

func sendError(ctx *RequestCtx, statusCode int, msg string) {
	if len(msg) == 0 {
		msg = fasthttp.StatusMessage(statusCode)
	}
	ctx.Error(msg, statusCode)
	return
}
