/*
Golang implementation of a rundown protection for accessing a shared object

Source code and other details for the project are available at GitHub:

	https://github.com/RandLabs/rundown-protection

More usage please see README.md and tests.
*/

package rundown_protection

import (
	"sync/atomic"
)

//------------------------------------------------------------------------------

const (
	rundownActive uint32 = 0x80000000
)

//------------------------------------------------------------------------------

type RundownProtection struct {
	counter uint32
	done chan struct{}
}

//------------------------------------------------------------------------------

func Create() *RundownProtection {
	r := &RundownProtection{}
	r.Initialize()
	return r
}

func (r *RundownProtection) Initialize() {
	atomic.StoreUint32(&r.counter, 0)
	r.done = make(chan struct{}, 1)
	return
}

func (r *RundownProtection) Acquire() bool {
	for {
		val := atomic.LoadUint32(&r.counter)
		if (val & rundownActive) != 0 {
			return false
		}

		if atomic.CompareAndSwapUint32(&r.counter, val, val + 1) {
			break
		}
	}
	return true
}

func (r *RundownProtection) Release() {
	for {
		val := atomic.LoadUint32(&r.counter)
		newVal := (val & rundownActive) | ((val & (^rundownActive)) - 1)
		if atomic.CompareAndSwapUint32(&r.counter, val, newVal) {
			if newVal == rundownActive {
				r.done <- struct{}{}
			}
			break
		}
	}
	return
}

func (r *RundownProtection) Wait() {
	var val uint32

	for {
		val = atomic.LoadUint32(&r.counter)
		if atomic.CompareAndSwapUint32(&r.counter, val, val | rundownActive) {
			break
		}
	}

	//wait
	if val != 0 {
		<-r.done
	}
	return
}
