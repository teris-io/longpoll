// Copyright 2015 Ventu.io. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file

package lpoll

import (
	"github.com/satori/go.uuid"
	"sync"
	"sync/atomic"
	"time"
)

type Sub struct {
	mx       sync.Mutex
	id       string
	polltime time.Duration
	onClose  func(id string)
	topics   map[string]bool
	data     []interface{}
	alive    int32
	notif    *getnotifier
	tor      *Timeout
}

type getnotifier struct {
	ping   chan bool
	pinged bool
}

func NewSub(timeout time.Duration, polltime time.Duration, onClose func(id string), topics ...string) *Sub {
	log.Info("new Subscription(%v, %v, %v, %v)", timeout, polltime, onClose, topics)
	sub := &Sub{
		id:       uuid.NewV4().String(),
		polltime: polltime,
		onClose:  onClose,
		topics:   make(map[string]bool),
		alive:    yes,
		notif:    nil,
	}
	for _, topic := range topics {
		sub.topics[topic] = true
	}
	sub.tor = NewTimeout(timeout, sub.Drop)
	return sub
}

// Publish appends supplied data to the internal storage of the subscription
// channel if the topic is one of subscribed to and the channel is alive.
func (sub *Sub) Publish(data interface{}, topic string) {
	if _, ok := sub.topics[topic]; !ok || !sub.IsAlive() {
		return
	}
	go func() {
		sub.mx.Lock()
		defer sub.mx.Unlock()

		// subscription could have died between the early check and entering the lock
		if sub.IsAlive() {
			sub.data = append(sub.data, data)
			if sub.notif != nil && !sub.notif.pinged {
				sub.notif.ping <- true
				sub.notif.pinged = true
			}
		}
	}()
}

func (sub *Sub) Get() chan []interface{} {
	resp := make(chan []interface{}, 1)
	if !sub.IsAlive() {
		resp <- nil
		return resp
	}

	// TODO beautify and add explanatory comments
	go func() {
		log.Debug("incoming get request")
		sub.tor.Ping()

		sub.mx.Lock()
		// subscription could have died between the early check and entering the lock
		if !sub.IsAlive() {
			resp <- nil
			sub.mx.Unlock()
			return
		}
		log.Debug("incoming get request")

		// earlier "get" won't be notified any longer
		sub.notif = nil

		if len(sub.data) > 0 {
			resp <- sub.data
			sub.data = nil
			sub.mx.Unlock()
			return
		}

		notif := &getnotifier{ping: make(chan bool, 1), pinged: false}
		sub.notif = notif

		sub.mx.Unlock()

		gotdata := no

		pollend := make(chan bool, 1)
		go func() {
			endpoint := time.Now().Add(sub.polltime)
			// let it quit much quicker
			hundredth := sub.polltime / 100
			for atomic.LoadInt32(&gotdata) == no && time.Now().Before(endpoint) {
				time.Sleep(hundredth)
			}
			pollend <- true
		}()

		select {
		case <-notif.ping:
			atomic.StoreInt32(&gotdata, yes)
			sub.mx.Lock()
			ndata := len(sub.data)
			resp <- sub.data
			sub.data = nil
			if sub.notif == notif {
				sub.notif = nil
			}
			sub.mx.Unlock()
			log.Debug("get received %v data objects", ndata)

		case <-pollend:
			sub.mx.Lock()
			resp <- nil
			if sub.notif == notif {
				sub.notif = nil
			}
			sub.mx.Unlock()
			log.Debug("get long poll ended empty")
		}
	}()
	return resp
}

func (sub *Sub) IsAlive() bool {
	return atomic.LoadInt32(&sub.alive) == yes
}

func (sub *Sub) Drop() {
	if !sub.IsAlive() {
		return
	}
	atomic.StoreInt32(&sub.alive, no)
	log.Notice("dropping subscription %v", sub.id)

	go func() {
		// prevent any external changes to data, new subscriptions
		sub.mx.Lock()
		defer sub.mx.Unlock()

		// signal timeout handler to quit
		sub.tor.Drop()
		// clear topics: no publishing possible
		sub.topics = make(map[string]bool)
		// clear data: no subscription gets anything
		sub.data = nil
		// let current get know that it should quit (with no data, see above)
		if sub.notif != nil && !sub.notif.pinged {
			sub.notif.ping <- true
		}
		// tell publish that there is no get listening, let it quit
		sub.notif = nil
		// execute callback (e.g. removing from pubsub subscriptions map)
		if sub.onClose != nil {
			sub.onClose(sub.id)
		}
	}()
}

func (sub *Sub) Id() string {
	return sub.id
}

func (sub *Sub) Topics() []string {
	// do not synchronise, value only changes on drop
	var res []string
	for topic, _ := range sub.topics {
		res = append(res, topic)
	}
	return res
}

func (sub *Sub) QueueSize() int {
	// do not synchronise
	return len(sub.data)
}

func (sub *Sub) GetWaiting() bool {
	// do not synchronise
	return sub.notif != nil
}
