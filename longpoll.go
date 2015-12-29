// Copyright 2015 Ventu.io. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file

package longpoll

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type LongPoll struct {
	mx    sync.Mutex
	subs  map[string]*Sub
	alive int32
}

func New() *LongPoll {
	return &LongPoll{
		subs:  make(map[string]*Sub),
		alive: yes,
	}
}

func (lp *LongPoll) Publish(data interface{}, topics ...string) {
	// lock for iterating over map
	lp.mx.Lock()
	defer lp.mx.Unlock()
	for _, topic := range topics {
		for _, sub := range lp.subs {
			go sub.Publish(data, topic)
		}
	}
}

func (lp *LongPoll) Subscribe(timeout time.Duration, topics ...string) string {
	sub := NewSub(timeout, func(id string) {
		// lock for deletion
		lp.mx.Lock()
		delete(lp.subs, id)
		lp.mx.Unlock()
	}, topics...)
	// lock for element insertion
	lp.mx.Lock()
	lp.subs[sub.id] = sub
	lp.mx.Unlock()
	return sub.id
}

func (lp *LongPoll) Get(id string, polltime time.Duration) (chan []interface{}, error) {
	// do not lock
	if sub, ok := lp.subs[id]; ok {
		return sub.Get(polltime), nil
	} else {
		return nil, errors.New("incorrect subscription id")
	}
}

func (lp *LongPoll) Drop(id string) {
	if sub, ok := lp.subs[id]; ok {
		go sub.Drop()
		// lock for deletion
		lp.mx.Lock()
		delete(lp.subs, id)
		lp.mx.Unlock()
	}
}

func (lp *LongPoll) Shutdown() {
	atomic.StoreInt32(&lp.alive, no)
	// lock for iterating over map
	lp.mx.Lock()
	defer lp.mx.Unlock()
	for id, sub := range lp.subs {
		go sub.Drop()
		delete(lp.subs, id)
	}
}

func (lp *LongPoll) List() []string {
	// lock for iterating over map
	lp.mx.Lock()
	defer lp.mx.Unlock()
	var res []string
	for id, _ := range lp.subs {
		res = append(res, id)
	}
	return res
}

func (lp *LongPoll) Topics() []string {
	topics := make(map[string]bool)
	// lock for iterating over map
	lp.mx.Lock()
	for _, sub := range lp.subs {
		for topic, _ := range sub.topics {
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

// IsAlive tests if the subscription is up and running.
func (lp *LongPoll) IsAlive() bool {
	return atomic.LoadInt32(&lp.alive) == yes
}
