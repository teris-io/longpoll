package longpoll

import (
	"sync/atomic"
	"time"
)

type Timeout struct {
	lastping int64
	report   chan bool
}

func NewTimeout(timeout time.Duration) *Timeout {
	tor := Timeout{
		report: make(chan bool, 1),
	}
	tor.Ping()
	go tor.start(int64(timeout))
	return &tor
}

func (tor *Timeout) Ping() {
	atomic.StoreInt64(&tor.lastping, tor.now())
}

func (tor *Timeout) ReportChan() chan bool {
	return tor.report
}

func (tor *Timeout) Drop() {
	atomic.StoreInt64(&tor.lastping, 0)
}

func (tor *Timeout) start(timeout int64) {
	for tor.elapsed() < timeout {
		time.Sleep(1)
	}
	tor.report <- true
}

func (tor *Timeout) elapsed() int64 {
	return tor.now() - atomic.LoadInt64(&tor.lastping)
}

func (tor *Timeout) now() int64 {
	return time.Now().UnixNano()
}
