// Copyright (c) 2015 Ventu.io, Oleg Sklyar, contributors
// The use of this source code is governed by a MIT style license found in the LICENSE file

package longpoll

import (
	"errors"
	"github.com/ventu-io/go-shortid"
	"github.com/ventu-io/slf"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Channel represents a single channel for publishing and receiving data over a long-polling
// subscription. Data published to any of the topics subscribed to will be received by the client
// asking for new data. The receiving is not split by topic.
//
// The subscription is setup to timeout if no Get request is made before the end of the timeout
// period provided at construction. Every Get request extends the lifetime of the subscription for
// the duration of the timeout.
type Channel struct {
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

// NewChannel constructs a new long-polling pubsub channel with the given timeout, optional exit
// handler, and subscribing to given topics. Every new channel gets a unique channel/subscription Id
// assigned based on UUID.v4.
//
// Constructing a channel with NewChannel starts a timeout timer. The first Get request must
// follow within the timeout window.
func NewChannel(timeout time.Duration, onClose func(id string), topics ...string) (*Channel, error) {
	if len(topics) == 0 {
		return nil, errors.New("at least one topic expected")
	}
	id, err := shortid.Generate()
	if err != nil {
		return nil, err
	}
	ch := Channel{
		id:      id,
		onClose: onClose,
		topics:  make(map[string]bool),
		alive:   yes,
	}
	for _, topic := range topics {
		ch.topics[topic] = true
	}
	if tor, err := NewTimeout(timeout, ch.Drop); err == nil {
		ch.tor = tor
	} else {
		return nil, err
	}
	logger.WithFields(slf.Fields{
		"id":      ch.id,
		"topics":  topics,
		"timeout": timeout,
		"onClose": onClose,
	}).Info("new subscription channel")
	return &ch, nil
}

// MustNewChannel acts just like NewChannel, however, it does not return
// errors and panics instead.
func MustNewChannel(timeout time.Duration, onClose func(id string), topics ...string) *Channel {
	ch, err := NewChannel(timeout, onClose, topics...)
	if err == nil {
		return ch
	}
	panic(err)
}

// Publish publishes data on the channel in a non-blocking manner if the topic corresponds to one of
// those provided at construction. Data published to other topics will be silently ignored. No topic
// information is persisted and retrieved with the data.
func (ch *Channel) Publish(data interface{}, topic string) error {
	if !ch.IsAlive() {
		return errors.New("subscription channel is down")
	}
	// no locking: read-only upon construction
	if _, ok := ch.topics[topic]; !ok {
		return nil
	}
	go func() {
		ch.mx.Lock()
		defer ch.mx.Unlock()

		// ch could have died between the check above and entering the lock
		if ch.IsAlive() {
			ch.data = append(ch.data, data)
			if ch.notif != nil && !ch.notif.pinged {
				ch.notif.pinged = true
				ch.notif.ping <- true
			}
		}
	}()
	// this routine is likely to be run within a goroutine and in case of non-stop publishing Gets may
	// have little chance to receive data otherwise
	defer runtime.Gosched()
	return nil
}

// Get requests data published on all of the channel topics. The function returns a channel
// to receive the data set on.
//
// The request is held until data becomes available (published to a matching topic). Upon new data,
// or if data has been waiting at the time of the call, the request returns immediately. Otherwise
// it waits over the `polltime` duration and return empty if no new data arrives. It is expected
// that a new Get request is made immediately afterwards to receive further data and prevent channel
// timeout.
//
// Multiple Get requests to the channel can be made concurrently, however, every data sample
// will be delivered to only one request issuer. It is not guaranteed to which one, although
// every new incoming request will trigger a return of any earlier one.
func (ch *Channel) Get(polltime time.Duration) (chan []interface{}, error) {
	if !ch.IsAlive() {
		return nil, errors.New("subscription channel is down")
	}
	if polltime <= 0 {
		return nil, errors.New("positive polltime value expected")
	}
	resp := make(chan []interface{}, 1)
	go func() {
		ch.tor.Ping()
		ch.mx.Lock()
		// ch could have died between the check above and entering the lock
		if !ch.IsAlive() {
			// next request will result in an error
			resp <- nil
			ch.mx.Unlock()
			return
		}
		logger.Debug("incoming get request")
		// notify existing Get to terminate immediately (will wait for lock)
		if ch.notif != nil && !ch.notif.pinged {
			ch.notif.pinged = true
			ch.notif.ping <- true
		}

		// ch.notif is reset either here, ...
		if ch.onDataWaiting(resp) {
			ch.mx.Unlock()
			return
		}

		// ...or here. Set this one to be notified by Publish
		notif := &getnotifier{ping: make(chan bool, 1), pinged: false}
		ch.notif = notif
		ch.mx.Unlock()

		gotdata := no
		pollend := make(chan bool, 1)

		go ch.startLongpollTimer(polltime, pollend, &gotdata)

		select {
		case <-notif.ping:
			ch.onNewDataLocking(resp, notif)
		case <-pollend:
			ch.onLongpollTimeoutLocking(resp, notif)
		}

		// signal the long-poll timer to stop
		atomic.StoreInt32(&gotdata, yes)
	}()
	return resp, nil
}

func (ch *Channel) startLongpollTimer(polltime time.Duration, pollend chan bool, gotdata *int32) {
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

func (ch *Channel) onDataWaiting(resp chan []interface{}) bool {
	if len(ch.data) > 0 {
		// answer with currently waiting data
		resp <- ch.data
		ndata := len(ch.data)
		logger.WithField("objects", ndata).Debug("sending data to waiting get")
		// remove data as it is already sent back
		ch.data = nil
		// earlier Get should get nothing, this one comes back with data immediately,
		// thus no get notifier for Publish
		ch.notif = nil
		return true
	}
	return false
}

func (ch *Channel) onNewDataLocking(resp chan []interface{}, notif *getnotifier) {
	ch.mx.Lock()
	defer ch.mx.Unlock()
	// answer with currently waiting data
	resp <- ch.data
	ndata := len(ch.data)
	logger.WithField("objects", ndata).Debug("sending data to waiting get")
	// remove data as it is already sent back
	ch.data = nil
	// remove this Get from Publish notification as this Get is already processed
	if ch.notif == notif {
		ch.notif = nil
	}
}

func (ch *Channel) onLongpollTimeoutLocking(resp chan []interface{}, notif *getnotifier) {
	ch.mx.Lock()
	defer ch.mx.Unlock()
	// asnwer with no data
	resp <- nil
	logger.Debug("get ended empty upon polltime")
	// remove this Get from Publish notification as this Get is already processed
	if ch.notif == notif {
		ch.notif = nil
	}
}

// IsAlive tests if the channel is up and running.
func (ch *Channel) IsAlive() bool {
	return atomic.LoadInt32(&ch.alive) == yes
}

// Drop terminates any publishing and receiving on the channel, signals the currently waiting Get
// request to return empty, terminates the timeout timer and runs the exit handler if supplied.
func (ch *Channel) Drop() {
	if !ch.IsAlive() {
		return
	}
	atomic.StoreInt32(&ch.alive, no)
	logger.WithField("id", ch.id).Warn("dropping subscription channel")

	go func() {
		// prevent any external changes to data, new subscriptions
		ch.mx.Lock()
		defer ch.mx.Unlock()

		// signal timeout handler to quit
		ch.tor.Drop()
		// clear data: no subscription gets anything
		ch.data = nil
		// let current get know that it should quit (with no data, see above)
		if ch.notif != nil && !ch.notif.pinged {
			ch.notif.ping <- true
		}
		// tell publish that there is no get listening, let it quit
		ch.notif = nil
		// execute callback (e.g. removing from pubsub subscriptions map)
		if ch.onClose != nil {
			ch.onClose(ch.id)
		}
	}()
}

// ID returns the channel/subscription Id assigned at construction.
func (ch *Channel) ID() string {
	return ch.id
}

// Topics returns the list of topics the channel is subscribed to.
func (ch *Channel) Topics() []string {
	var res []string
	// no locking: read-only upon construction
	for topic := range ch.topics {
		res = append(res, topic)
	}
	return res
}

// QueueSize returns the size of the currently waiting data queue (only not empty when no Get
// request waiting).
func (ch *Channel) QueueSize() int {
	ch.mx.Lock()
	res := len(ch.data)
	ch.mx.Unlock()
	return res
}

// IsGetWaiting reports if there is a Get request waiting for data.
func (ch *Channel) IsGetWaiting() bool {
	// do not synchronise
	return ch.notif != nil
}
