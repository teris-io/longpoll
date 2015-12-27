package longpoll

import (
	"errors"
	"github.com/satori/go.uuid"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const (
	no int32 = iota
	yes
)

type Subscription struct {
	mx       sync.Mutex
	id       string
	polltime time.Duration
	topics   map[string]bool
	data     []*interface{}
	cmd      chan interface{}
	alive    int32
	newdata  chan bool
}

func NewSubscription(timeout time.Duration, polltime time.Duration, onClose func(id string), topics ...string) *Subscription {
	topicmap := make(map[string]bool)
	for _, topic := range topics {
		topicmap[topic] = true
	}
	sub := Subscription{
		id:       uuid.NewV4().String(),
		polltime: polltime,
		topics:   topicmap,
		cmd:      make(chan interface{}, 100),
		alive:    yes,
		newdata:  nil,
	}
	go sub.start(timeout, onClose)
	return &sub
}

type pubinfo struct {
	data  *interface{}
	topic string
}

func (sub *Subscription) Publish(data *interface{}, topic string) {
	if sub.IsAlive() {
		// never block publishing
		go func() {
			sub.cmd <- pubinfo{data: data, topic: topic}
		}()
	}
}

type getinfo struct {
	resp     chan []*interface{}
	polltime time.Duration
}

func (sub *Subscription) Get() ([]*interface{}, error) {
	if sub.IsAlive() {
		// allow the data supplier exit even if we cannot get it
		ch := make(chan []*interface{}, 1)
		// block here shortly if required
		sub.cmd <- getinfo{resp: ch, polltime: sub.polltime}
		// block here until data or timeout signalled
		res, ok := <-ch
		if ok {
			return res, nil
		}
	}
	return nil, errors.New("subscription is closed")
}

func (sub *Subscription) IsAlive() bool {
	return atomic.LoadInt32(&sub.alive) == yes
}

func (sub *Subscription) Drop() {
	if sub.IsAlive() {
		// never block here
		go func() {
			sub.cmd <- true
		}()
	}
}

func (sub *Subscription) start(timeout time.Duration, onClose func(id string)) {
	tor := NewTimeout(timeout)

CommandLoop:
	for {
		select {
		case <-tor.ReportChan():
			break CommandLoop

		case cmd, ok := <-sub.cmd:
			if !ok {
				break CommandLoop
			}
			switch cmd.(type) {
			case pubinfo:
				pinfo, _ := cmd.(pubinfo)
				go sub.pub(&pinfo)

			case getinfo:
				tor.Ping()
				ginfo, _ := cmd.(getinfo)
				go sub.get(&ginfo)

			case bool:
				break CommandLoop

			default:
				panic("unexpected information passed to sub cmd")
			}

		}
	}

	atomic.StoreInt32(&sub.alive, no)

	tor.Drop()
	if onClose != nil {
		go onClose(sub.id)
	}
}

func (sub *Subscription) pub(pinfo *pubinfo) {
	if _, ok := sub.topics[pinfo.topic]; ok {
		sub.mx.Lock()
		sub.data = append(sub.data, pinfo.data)
		sub.mx.Unlock()
	}
}

// TODO find a way to
func (sub *Subscription) get(ginfo *getinfo) {
	var res []*interface{}
	start := time.Now()

	// FIXME will this endless loop cause CPU usage?
	for time.Now().Sub(start) < ginfo.polltime && sub.IsAlive() {
		if len(sub.data) > 0 {
			sub.mx.Lock()
			res = sub.data
			sub.data = nil
			sub.mx.Unlock()
			// earlier check outside of the lock, so we could potentially take over empty results
			if len(res) > 0 {
				break
			}
		} else {
			runtime.Gosched()
		}
	}
	ginfo.resp <- res
}
