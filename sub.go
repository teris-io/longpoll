// Copyright 2015 Ventu.io. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file

package longpoll

import (
	"github.com/satori/go.uuid"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Sub represents a long-poll subscription. Data can be published to any topic
// the subscription was initialised for. The retrieval is not topic specific.
//
// If data has already been published by the time the Get request is made,
// the subscription will immediately return the data back. Otherwise, the Get
// request will wait until data is published or long-poll timeout occurs,
// in which case Get returns an empty data set.
//
// If multiple data instances are published while a Get request is waiting,
// there is no contract for that Get request to receive exactly one or more
// than one instances of data. Subsequent Get requests may be required to
// receive all the data. If another Get request is made while one is waiting
// the latter (the one issued earlier) is most likely to return no data with
// all data delivered in the later request. It is not advisable to issue
// concurrent Get requests to the same subscription, but overall no data
// loss is expected.
//
// The subscription is setup to time out if no Get request is issued during the
// timeout period provided at construction. Every Get request extends the
// lifetime of the subscription for another duration of the initial timeout.
type Sub struct {
	mx      sync.Mutex
	id      string
	onClose func(id string)
	topics  map[string]bool
	data    []interface{}
	alive   int32
	notif   *getnotifier
	tor     *Timeout
}

type getnotifier struct {
	ping   chan bool
	pinged bool
}

// NewSub constructs a new subscription with the given timeout, exit handler
// (can be nil), and a collection of topics. Only data published to those
// topics will be delivered to clients.
//
// Every new subscription gets a unique Id assigned based on UUIDv.4.
//
// Constructing a subscription with NewSub will start the timeout timer.
func NewSub(timeout time.Duration, onClose func(id string), topics ...string) *Sub {
	log.Info("new Subscription(%v, %v, %v)", timeout, onClose, topics)
	sub := &Sub{
		id:      uuid.NewV4().String(),
		onClose: onClose,
		topics:  make(map[string]bool),
		alive:   yes,
		notif:   nil,
	}
	for _, topic := range topics {
		sub.topics[topic] = true
	}
	sub.tor = NewTimeout(timeout, sub.Drop)
	return sub
}

// Publish delivers data in a non-blocking manner to the currently waiting
// Get request or memorises it for Get requests issued later but before the
// timeout. Only data published to topics that the subscription is subscribed
// to gets published. Other data will be silently ignored.
func (sub *Sub) Publish(data interface{}, topic string) {
	if _, ok := sub.topics[topic]; !ok || !sub.IsAlive() {
		return
	}
	go func() {
		sub.mx.Lock()
		defer sub.mx.Unlock()

		// sub could have died between the check above and entering the lock
		if sub.IsAlive() {
			sub.data = append(sub.data, data)
			if sub.notif != nil && !sub.notif.pinged {
				sub.notif.pinged = true
				sub.notif.ping <- true
			}
		}
	}()
	runtime.Gosched()
}

// Get requests for data published on the topics subscribed to. It i not topic-
// specific. If data is already available and waiting, the request will collect
// it and return immediately. Otherwise, the request will wait for new data
// published to the topics subscribed to and will return it directly after it is
// published.
//
// If no new data arrives before the end of polltime, the request returns an
// empty data set. It is expected that a new Get request is scheduled immediately
// after the earlier one returns.
//
// If a concurrent Get request is issued while another one is waiting for data,
// the earlier one is likely to return an empty data set. The library only
// guarantees that there is no data loss over all Get requests in total.
func (sub *Sub) Get(polltime time.Duration) chan []interface{} {
	resp := make(chan []interface{}, 1)
	if !sub.IsAlive() {
		resp <- nil
		return resp
	}
	go func() {
		sub.tor.Ping()

		sub.mx.Lock()
		// sub could have died between the check above and entering the lock
		if !sub.IsAlive() {
			resp <- nil
			sub.mx.Unlock()
			return
		}

		log.Debug("incoming get request")

		if sub.onDataWaiting(resp) {
			sub.mx.Unlock()
			return
		}

		// prevent existing Get receive any new data and set this one to be notified by Publish
		notif := &getnotifier{ping: make(chan bool, 1), pinged: false}
		sub.notif = notif
		sub.mx.Unlock()

		gotdata := no
		pollend := make(chan bool, 1)

		go sub.startLongpollTimer(polltime, pollend, &gotdata)

		select {
		case <-notif.ping:
			sub.onNewDataLocking(resp, notif)
		case <-pollend:
			sub.onLongpollTimeoutLocking(resp, notif)
		}

		// signal the long-poll timer to stop
		atomic.StoreInt32(&gotdata, yes)
	}()
	return resp
}

func (sub *Sub) startLongpollTimer(polltime time.Duration, pollend chan bool, gotdata *int32) {
	hundredth := polltime / 100
	endpoint := time.Now().Add(polltime)
	for time.Now().Before(endpoint) {
		// if Get has data, this timer is irrelevant
		if atomic.LoadInt32(gotdata) == yes {
			return
		}
		// splitting polltime into 100 segments, let it quit much quicker
		time.Sleep(hundredth)
	}
	pollend <- true
}

func (sub *Sub) onDataWaiting(resp chan []interface{}) bool {
	if len(sub.data) > 0 {
		// answer with currently waiting data
		resp <- sub.data
		ndata := len(sub.data)
		log.Debug("get received %v data objects", ndata)
		// remove data as it is already sent back
		sub.data = nil
		// earlier Get should get nothing, this one comes back with data immediately,
		// thus no getnotifier for Publish
		sub.notif = nil
		return true
	} else {
		return false
	}
}

func (sub *Sub) onNewDataLocking(resp chan []interface{}, notif *getnotifier) {
	sub.mx.Lock()
	defer sub.mx.Unlock()
	// answer with currently waiting data
	resp <- sub.data
	ndata := len(sub.data)
	log.Debug("get received %v data objects", ndata)
	// remove data as it is already sent back
	sub.data = nil
	// remove this Get from Publish notification as this Get is already processed
	if sub.notif == notif {
		sub.notif = nil
	}
}

func (sub *Sub) onLongpollTimeoutLocking(resp chan []interface{}, notif *getnotifier) {
	sub.mx.Lock()
	defer sub.mx.Unlock()
	// asnwer with no data
	resp <- nil
	log.Debug("get long poll ended empty")
	// remove this Get from Publish notification as this Get is already processed
	if sub.notif == notif {
		sub.notif = nil
	}
}

// IsAlive tests if the subscription is up and running.
func (sub *Sub) IsAlive() bool {
	return atomic.LoadInt32(&sub.alive) == yes
}

// Drop terminates the subscription and removes the topics (nothing can
// then be published), signals the currently waiting Get request to return
// empty, terminates the timeout timer and runs the exit handler if supplied.
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

// Id returns the subscription Id assigned at construction.
func (sub *Sub) Id() string {
	return sub.id
}

// Topics return the current list of topics (fixed unless Drop
// is called, which resets topics to an empty list).
func (sub *Sub) Topics() []string {
	// do not synchronise, value only changes on drop
	var res []string
	for topic, _ := range sub.topics {
		res = append(res, topic)
	}
	return res
}

// QueueSize reports a snapshot of the size of the currently waiting
// data queue (which is only not empty if no Get request waiting).
func (sub *Sub) QueueSize() int {
	// do not synchronise
	return len(sub.data)
}

// IsGetWaiting reports if there is a Get request waiting for data. The
// result may be nondeterministic during continuous publishing.
func (sub *Sub) IsGetWaiting() bool {
	// do not synchronise
	return sub.notif != nil
}
