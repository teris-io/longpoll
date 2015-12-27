package longpoll

import (
	"sync/atomic"
	"time"
)

type Timeout struct {
	lastping  int64
	alive     int32
	report    chan bool
	onTimeout func()
}

func NewTimeout(timeout time.Duration, onTimeout func()) *Timeout {
	tor := Timeout{
		alive:     YES,
		report:    make(chan bool, 1),
		onTimeout: onTimeout,
	}
	tor.Ping()
	go tor.start(int64(timeout))
	return &tor
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
	atomic.StoreInt32(&tor.alive, NO)
}

func (tor *Timeout) start(timeout int64) {
	for tor.elapsed() < timeout && tor.IsAlive() {
		time.Sleep(1)
	}
	tor.report <- true
	if tor.IsAlive() && tor.onTimeout != nil {
		go tor.onTimeout()
	}
	tor.Drop()
}

func (tor *Timeout) IsAlive() bool {
	return atomic.LoadInt32(&tor.alive) == YES
}

func (tor *Timeout) elapsed() int64 {
	return tor.now() - atomic.LoadInt64(&tor.lastping)
}

func (tor *Timeout) now() int64 {
	return time.Now().UnixNano()
}
