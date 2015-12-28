// Copyright 2015 Ventu.io. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file

package longpoll

import (
	"sync/atomic"
	"time"
)

// Timeout implements a callback mechanism on timeout (along with
// reporting on a buffered channel), which is extendable in time via
// pinging the object. An alive timeout can be dropped at any time,
// in which case the callback will not be executed, but the exit
// will still be reported on the channel.
//
// This extendable Timeout is used for monitoring long polling
// subscriptions here, which would expire if no client asks for data
// within a defined timeout (or timeout extended otherwise).
type Timeout struct {
	lastping  int64
	alive     int32
	report    chan bool
	onTimeout func()
}

// NewTimeout creates and starts a new timeout handler accepting the
// duration of
func NewTimeout(timeout time.Duration, onTimeout func()) *Timeout {
	log.Debug("new Timeout(%v, %v)", timeout, onTimeout)
	tor := &Timeout{
		alive:     yes,
		report:    make(chan bool, 1),
		onTimeout: onTimeout,
	}
	tor.Ping()
	go tor.handle(int64(timeout))
	return tor
}

func (tor *Timeout) Ping() {
	if tor.IsAlive() {
		atomic.StoreInt64(&tor.lastping, tor.now())
	}
}

func (tor *Timeout) ReportChan() chan bool {
	return tor.report
}

func (tor *Timeout) Drop() {
	atomic.StoreInt32(&tor.alive, no)
}

func (tor *Timeout) handle(timeout int64) {
	log.Debug("handler started")
	hundredth := timeout / 100
	for tor.elapsed() < timeout && tor.IsAlive() {
		time.Sleep(time.Duration(hundredth))
	}
	if tor.IsAlive() {
		log.Notice("timeout detected")
		tor.Drop()
		if tor.onTimeout != nil {
			log.Debug("calling onTimeout handler")
			go tor.onTimeout()
		}
	}
	log.Debug("reporting exit on channel")
	tor.report <- true
}

func (tor *Timeout) IsAlive() bool {
	return atomic.LoadInt32(&tor.alive) == yes
}

func (tor *Timeout) elapsed() int64 {
	return tor.now() - atomic.LoadInt64(&tor.lastping)
}

func (tor *Timeout) now() int64 {
	return time.Now().UnixNano()
}
