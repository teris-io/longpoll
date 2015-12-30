// Copyright 2015 Ventu.io. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file

// The package `longpoll` provides an implementation of the long-polling mechanism of the
// PubSub pattern. Although the primary purpose of the package is to aid the development
// of web applications, it provides no specific web handlers and  can be used in other
// distributed applications.
//
// The package provides the `Channel` type to manage individual long-polling subscriptions and the
// `LongPoll` type to manage subscriptions set allowing for adding and removing subscriptions.
package longpoll

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// The LongPoll type represents a subscription manager. It provides functionality to manage multiple
// long-polling subscriptions allowing for adding and removing subscriptions, publishing to all
// subscriptions, receiving data by subscription Id.
type LongPoll struct {
	mx       sync.Mutex
	channels map[string]*Channel
	alive    int32
}

// New creates a new long-polling subscription manager.
func New() *LongPoll {
	return &LongPoll{
		channels: make(map[string]*Channel),
		alive:    yes,
	}
}

// Publish publishes data on all the subscribed channels with minimal blocking. Data is published
// separately to each topic. Closed channels and mismatching topics are ignored silently.
func (lp *LongPoll) Publish(data interface{}, topics ...string) {
	if len(topics) == 0 {
		return
	}

	for _, topic := range topics {
		lp.mx.Lock()
		for _, ch := range lp.channels {
			ch.Publish(data, topic) // errors ignored
		}
		lp.mx.Unlock()
	}
}

func (lp *LongPoll) Subscribe(timeout time.Duration, topics ...string) (string, error) {
	if ch, err := NewChannel(timeout, lp.drop, topics...); err == nil {
		lp.mx.Lock()
		lp.channels[ch.id] = ch
		lp.mx.Unlock()
		return ch.id, nil
	} else {
		return "", err
	}
}

func (lp *LongPoll) drop(id string) {
	lp.mx.Lock()
	delete(lp.channels, id)
	lp.mx.Unlock()
}

func (lp *LongPoll) Channel(id string) (*Channel, bool) {
	lp.mx.Lock()
	res, ok := lp.channels[id]
	lp.mx.Unlock()
	return res, ok
}

func (lp *LongPoll) Channels() []*Channel {
	var res []*Channel
	lp.mx.Lock()
	for _, ch := range lp.channels {
		res = append(res, ch)
	}
	lp.mx.Unlock()
	return res
}

func (lp *LongPoll) Ids() []string {
	var res []string
	lp.mx.Lock()
	for id, _ := range lp.channels {
		res = append(res, id)
	}
	lp.mx.Unlock()
	return res
}

func (lp *LongPoll) Get(id string, polltime time.Duration) (chan []interface{}, error) {
	if ch, ok := lp.Channel(id); ok {
		return ch.Get(polltime)
	} else {
		return nil, errors.New(fmt.Sprintf("no channel for Id %v", id))
	}
}

// IsAlive tests if the subscription is up and running.
func (lp *LongPoll) IsAlive() bool {
	return atomic.LoadInt32(&lp.alive) == yes
}

func (lp *LongPoll) Drop(id string) {
	if ch, ok := lp.Channel(id); ok {
		ch.Drop()
	}
}

func (lp *LongPoll) Shutdown() {
	atomic.StoreInt32(&lp.alive, no)
	channels := lp.Channels()
	for _, ch := range channels {
		ch.Drop()
	}
}

func (lp *LongPoll) Topics() []string {
	topics := make(map[string]bool)
	lp.mx.Lock()
	for _, ch := range lp.channels {
		for topic, _ := range ch.topics {
			topics[topic] = true
		}
	}
	lp.mx.Unlock()

	var res []string
	for topic, _ := range topics {
		res = append(res, topic)
	}
	return res
}
