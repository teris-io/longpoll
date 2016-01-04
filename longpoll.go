// Copyright (c) 2015 Ventu.io, Oleg Sklyar, contributors
// The use of this source code is governed by a MIT style license found in the LICENSE file

// Package longpoll provides an implementation of the long polling mechanism of the PubSub
// pattern. Although the primary purpose of the package is to aid the development of web
// applications, it provides no specific web handlers and can be used in other distributed
// applications.
//
// The package provides the Channel type to manage publishing and retrieval of information for each
// individual subscription, and the LongPoll type to manage subscription channels allowing for
// adding, removing and publishing to all.
package longpoll

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// The LongPoll type represents a subscription manager. It provides functionality to manage multiple
// long-polling subscriptions allowing for adding and removing subscriptions, publishing to all
// subscriptions, receiving data by subscription Id.
type LongPoll struct {
	mx    sync.Mutex
	chmap map[string]*Channel
	alive int32
	// performance optimisation: channel list cache between updates to avoid reconstructing it
	// from chmap values and unlocking the thread ASAP. Reset to nil on any alterations to chmap
	chcache []*Channel
}

// New creates a new long-polling subscription manager.
func New() *LongPoll {
	return &LongPoll{
		chmap: make(map[string]*Channel),
		alive: yes,
	}
}

// Subscribe creates a new subscription channel and returns its Id (and an error if the subscription
// channel could not be created). The subscription channel is automatically open to publishing.
func (lp *LongPoll) Subscribe(timeout time.Duration, topics ...string) (string, error) {
	if !lp.IsAlive() {
		return "", errors.New("pubsub is down")
	}
	ch, err := NewChannel(timeout, lp.drop, topics...)
	if err == nil {
		lp.mx.Lock()
		lp.chcache = nil
		lp.chmap[ch.id] = ch
		lp.mx.Unlock()
		return ch.id, nil
	}
	return "", err
}

// MustSubscribe acts in the same manner as Subscribe, however, it does not return errors
// and panics instead.
func (lp *LongPoll) MustSubscribe(timeout time.Duration, topics ...string) string {
	id, err := lp.Subscribe(timeout, topics...)
	if err == nil {
		return id
	}
	panic(err)
}

// Publish publishes data on all subscription channels with minimal blocking. Data is published
// separately for each topic. Closed subscription channels and mismatching topics are ignored silently.
func (lp *LongPoll) Publish(data interface{}, topics ...string) error {
	if !lp.IsAlive() {
		return errors.New("pubsub is down")
	}
	if len(topics) == 0 {
		return errors.New("expected at least one topic")
	}
	for _, ch := range lp.Channels() {
		for _, topic := range topics {
			ch.Publish(data, topic) // errors ignored
		}
	}
	return nil
}

// Channel returns a pointer to the subscription channel behind the given id.
func (lp *LongPoll) Channel(id string) (*Channel, bool) {
	if !lp.IsAlive() {
		return nil, false
	}
	lp.mx.Lock()
	res, ok := lp.chmap[id]
	lp.mx.Unlock()
	return res, ok && res.IsAlive()
}

// Channels returns the list of all currently up and running subscription channels. For performance
// reasons when dealing with a large number of subscription channels all operations across all of
// them use this method to retrieve the list first and unlock the thread ASAP. If a subscription
// channel is removed after the list was retrieved, the operation will still run on that channel. If
// a channel is added, the operation will not apply to it.
func (lp *LongPoll) Channels() []*Channel {
	if !lp.IsAlive() {
		return nil
	}

	lp.mx.Lock()
	defer lp.mx.Unlock()

	if len(lp.chcache) == 0 { // either no data or invalidated
		for _, ch := range lp.chmap {
			if ch.IsAlive() {
				lp.chcache = append(lp.chcache, ch)
			}
		}
	}
	return lp.chcache
}

// Ids returns the list of Ids of all currently up and running subscription channels.
func (lp *LongPoll) Ids() []string {
	if !lp.IsAlive() {
		return nil
	}

	var res []string
	for _, ch := range lp.Channels() {
		if ch.IsAlive() {
			res = append(res, ch.ID())
		}
	}
	return res
}

// Get requests data published on all of the topics for the given subscription channel.
// See further info in (*Channel).Get.
func (lp *LongPoll) Get(id string, polltime time.Duration) (chan []interface{}, error) {
	if !lp.IsAlive() {
		return nil, errors.New("pubsub is down")
	}
	if ch, ok := lp.Channel(id); ok {
		return ch.Get(polltime)
	}
	return nil, fmt.Errorf("no channel for Id %v", id)
}

// IsAlive tests if the pubsub service is up and running.
func (lp *LongPoll) IsAlive() bool {
	return atomic.LoadInt32(&lp.alive) == yes
}

// Drop terminates a subscription channel for the given Id and removes it from
// the list of subscription channels.
func (lp *LongPoll) Drop(id string) {
	if ch, ok := lp.Channel(id); ok {
		// channel will call lp.drop if it is alive as it was given as exit handler
		// to be called on timeout (or any closure), however, we want to force it
		// even if channel is no more alive for any reasons:
		lp.drop(ch.ID())
		ch.Drop()
	}
}

func (lp *LongPoll) drop(id string) {
	lp.mx.Lock()
	lp.chcache = nil
	delete(lp.chmap, id)
	lp.mx.Unlock()
}

// Shutdown terminates the pubsub service and drops all subscription channels.
func (lp *LongPoll) Shutdown() {
	if !lp.IsAlive() {
		// already down (or going down) and this here is the only method that resets the flag
		return
	}

	atomic.StoreInt32(&lp.alive, no)

	lp.mx.Lock()
	defer lp.mx.Unlock()

	// do not use lp.Channels here as it delivers only alive ones
	for _, ch := range lp.chmap {
		ch.Drop()
	}
	// remove all subscription channels
	lp.chmap = make(map[string]*Channel)
	lp.chcache = nil
}

// Topics constructs the set of all topics, for which there are currently open
// subscription channels.
func (lp *LongPoll) Topics() []string {
	if !lp.IsAlive() {
		return nil
	}

	topics := make(map[string]bool)
	for _, ch := range lp.Channels() {
		if ch.IsAlive() {
			for topic := range ch.topics {
				topics[topic] = true
			}
		}
	}
	var res []string
	for topic := range topics {
		res = append(res, topic)
	}
	sort.Strings(res)
	return res
}
