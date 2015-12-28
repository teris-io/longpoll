// Copyright 2015 Ventu.io. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file

package longpoll

import (
	"github.com/satori/go.uuid"
	"sync"
	"sync/atomic"
	"time"
)

type Subscription struct {
	mx       sync.Mutex
	id       string
	polltime time.Duration
	onClose  func(id string)
	topics   map[string]bool
	data     []*interface{}
	alive    int32
	notif    *getnotifier
	tor      *Timeout
}

type getnotifier struct {
	ping   chan bool
	pinged bool
}

func NewSubscription(timeout time.Duration, polltime time.Duration, onClose func(id string), topics ...string) *Subscription {
	sub := &Subscription{
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
	log.Debug("new Subscription")
	sub.tor = NewTimeout(timeout, sub.Drop)
	return sub
}

// Publish appends supplied data to the internal storage of the subscription
// channel if the topic is one of subscribed to and the channel is alive.
func (sub *Subscription) Publish(data *interface{}, topic string) {
	if !sub.IsAlive() {
		return
	}
	if _, ok := sub.topics[topic]; !ok {
		return
	}
	go func() {
		sub.mx.Lock()
		defer sub.mx.Unlock()

		sub.data = append(sub.data, data)
		if sub.notif != nil && !sub.notif.pinged {
			sub.notif.ping <- true
			sub.notif.pinged = true
		}
	}()
}

func (sub *Subscription) Get() chan []*interface{} {
	resp := make(chan []*interface{}, 1)
	if !sub.IsAlive() {
		resp <- nil
		return resp
	}
	go func() {
		sub.tor.Ping()

		sub.mx.Lock()

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

		pollend := make(chan bool, 1)
		go func() {
			time.Sleep(sub.polltime)
			pollend <- true
		}()

		// TODO need an infinite loop here?

		select {
		case <-notif.ping:
			sub.mx.Lock()
			resp <- sub.data
			sub.data = nil
			if sub.notif == notif {
				sub.notif = nil
			}
			sub.mx.Unlock()

		case <-pollend:
			sub.mx.Lock()
			resp <- nil
			if sub.notif == notif {
				sub.notif = nil
			}
			sub.mx.Unlock()
		}
	}()
	return resp
}

func (sub *Subscription) IsAlive() bool {
	return atomic.LoadInt32(&sub.alive) == yes
}

func (sub *Subscription) Drop() {
	if !sub.IsAlive() {
		return
	}
	atomic.StoreInt32(&sub.alive, no)
	go func() {
		sub.tor.Drop()

		sub.mx.Lock()
		defer sub.mx.Unlock()

		sub.data = nil
		if sub.notif != nil && !sub.notif.pinged {
			sub.notif.ping <- true
		}
		sub.notif = nil
		if sub.onClose != nil {
			sub.onClose(sub.id)
		}
	}()
}
