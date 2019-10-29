package process

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

//------------------------------------------------------------------------------

var manualShutdownSignal chan os.Signal
var manualShutdownFired int32

//------------------------------------------------------------------------------

func init() {
	manualShutdownSignal = make(chan os.Signal, 1)
	return
}

func GetShutdownSignal() chan os.Signal {
	var osSignal chan os.Signal
	var internalSignal chan os.Signal

	osSignal = make(chan os.Signal, 1)
	internalSignal = make(chan os.Signal, 1)
	signal.Notify(internalSignal, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case s := <-internalSignal:
			osSignal <- s
			return

		case <-manualShutdownSignal:
			osSignal <- os.Interrupt
			return
		}
	}()

	return osSignal
}

func Shutdown() {
	if atomic.CompareAndSwapInt32(&manualShutdownFired, 0, 1) {
		manualShutdownSignal <- os.Interrupt
	}
}
