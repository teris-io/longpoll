package longpoll

import (
	"errors"
	"sync"
	"time"
)

type LongPoll struct {
	mx       sync.Mutex
	timeout  time.Duration
	polltime time.Duration
	subs     map[string]*Subscription
}

func New(timeout time.Duration, polltime time.Duration) *LongPoll {
	return &LongPoll{
		subs:     make(map[string]*Subscription),
		timeout:  timeout,
		polltime: polltime,
	}
}

func (lp *LongPoll) Publish(data *interface{}, topics ...string) {
	// lock for iterating over map
	lp.mx.Lock()
	defer lp.mx.Unlock()
	for _, topic := range topics {
		for _, sub := range lp.subs {
			go sub.Publish(data, topic)
		}
	}
}

func (lp *LongPoll) Subscribe(topics ...string) string {
	sub := NewSubscription(lp.timeout, lp.polltime, func(id string) {
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

func (lp *LongPoll) Get(id string) (chan []*interface{}, error) {
	// do not lock
	if sub, ok := lp.subs[id]; ok {
		return sub.Get(), nil
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
