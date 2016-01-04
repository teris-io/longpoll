// Copyright (c) 2015 Ventu.io, Oleg Sklyar, contributors
// The use of this source code is governed by a MIT style license found in the LICENSE file

package longpoll_test

import (
	"github.com/ventu-io/go-longpoll"
	"testing"
	"time"
)

func TestLongPoll_newFunctionWithStruct_inactive(t *testing.T) {
	ps := new(longpoll.LongPoll)
	if ps.IsAlive() {
		t.Error("the service is expected to be down")
	}
}

func TestLongPoll_onNewLongPoll_active(t *testing.T) {
	ps := longpoll.New()
	if !ps.IsAlive() {
		t.Error("the service is expected to be up")
	}
}

func TestLongPoll_onSubscribe_success(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	id1, err1 := ps.Subscribe(time.Minute, "A", "B")
	if err1 != nil {
		t.Error("expected no errors on creation")
	}
	id2, err2 := ps.Subscribe(time.Minute, "A", "B")
	if err2 != nil {
		t.Error("expected no errors on creation")
	}
	if id1 == id2 || len(id1) != 36 || len(id2) != 36 {
		t.Error("incorrect ids")
	}
}

func TestLongPoll_onMustSubscribe_success(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	id1 := ps.MustSubscribe(time.Minute, "A", "B")
	id2 := ps.MustSubscribe(time.Minute, "A", "B")
	if id1 == id2 || len(id1) != 36 || len(id2) != 36 {
		t.Error("incorrect ids")
	}
}

func TestLongPoll_onSubscribe_whenDown_error(t *testing.T) {
	ps := longpoll.New()
	ps.Shutdown()
	_, err1 := ps.Subscribe(time.Minute, "A", "B")
	if err1 == nil {
		t.Error("expected error on creation")
	}
}

func TestLongPoll_onMustSubscribe_whenDown_panics(t *testing.T) {
	ps := longpoll.New()
	ps.Shutdown()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()
	ps.MustSubscribe(time.Minute, "A", "B")
}

func TestLongPoll_onSubscribe_whenChannelError_error(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	var topics []string
	_, err1 := ps.Subscribe(time.Minute, topics...)
	if err1 == nil {
		t.Error("expected error on creation")
	}
}

func TestLongPoll_onPublish_success(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	id := ps.MustSubscribe(time.Minute, "A")
	ch, _ := ps.Channel(id)
	err := ps.Publish(make(map[string]int), "C")
	if err != nil {
		t.Error("no error expected")
	}
	time.Sleep(200 * time.Millisecond)
	if ch.QueueSize() != 0 {
		t.Error("no message expected")
	}
	err = ps.Publish(make(map[string]int), "A")
	if err != nil {
		t.Error("no error expected")
	}
	time.Sleep(200 * time.Millisecond)
	if ch.QueueSize() != 1 {
		t.Error("message expected")
	}
}

func TestLongPoll_onPublish_whenDown_error(t *testing.T) {
	ps := longpoll.New()
	ps.Shutdown()
	err := ps.Publish(make(map[string]int), "A")
	if err == nil {
		t.Error("expected error on publish")
	}
}

func TestLongPoll_onPublish_whenNoTopics_error(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	var topics []string
	err := ps.Publish(make(map[string]int), topics...)
	if err == nil {
		t.Error("expected error on publish")
	}
}

func TestLongPoll_onChannel_success(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	id := ps.MustSubscribe(time.Minute, "A")
	ch, ok := ps.Channel(id)
	if !ok {
		t.Error("channel expected")
	}
	if id != ch.Id() {
		t.Error("wrong id")
	}
}

func TestLongPoll_onChannel_whenDown_false(t *testing.T) {
	ps := longpoll.New()
	id := ps.MustSubscribe(time.Minute, "A")
	ps.Shutdown()
	_, ok := ps.Channel(id)
	if ok {
		t.Error("error expected")
	}
}

func TestLongPoll_onChannel_whenNoChannel_false(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	ps.MustSubscribe(time.Minute, "A")
	_, ok := ps.Channel("whatever")
	if ok {
		t.Error("error expected")
	}
}

func TestLongPoll_onChannel_whenChannelInactive_false(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	id := ps.MustSubscribe(time.Minute, "A")
	ch, _ := ps.Channel(id)
	ch.Drop()
	_, ok := ps.Channel(id)
	if ok {
		t.Error("error expected")
	}
}

func TestLongPoll_onChannels_success(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	if len(ps.Channels()) > 0 {
		t.Error("no channels expected")
	}
	ps.MustSubscribe(time.Minute, "A")
	id := ps.MustSubscribe(time.Minute, "A")
	ps.MustSubscribe(time.Minute, "A")
	time.Sleep(200 * time.Millisecond)
	if len(ps.Channels()) != 3 {
		t.Error("3 channels expected")
	}
	ps.Drop(id)
	time.Sleep(200 * time.Millisecond)
	if len(ps.Channels()) != 2 {
		t.Error("2 channels expected")
	}
	// cached
	if len(ps.Channels()) != 2 {
		t.Error("2 channels expected")
	}
}

func TestLongPoll_onChannels_whenDown_empty(t *testing.T) {
	ps := longpoll.New()
	ps.MustSubscribe(time.Minute, "A")
	ps.MustSubscribe(time.Minute, "A")
	ps.Shutdown()
	if len(ps.Channels()) > 0 {
		t.Error("no channels expected")
	}
}

func TestLongPoll_onIds_success(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	id1 := ps.MustSubscribe(time.Minute, "A")
	id2 := ps.MustSubscribe(time.Minute, "A")
	ids := ps.Ids()
	if id1 != ids[0] && id1 != ids[1] {
		t.Error("id1 must be present")
	}
	if id2 != ids[0] && id2 != ids[1] {
		t.Error("id2 must be present")
	}
	ps.Drop(id1)
	time.Sleep(200 * time.Millisecond)
	ids = ps.Ids()
	if len(ids) != 1 {
		t.Error("only 1 id is expected")
	}
}

func TestLongPoll_onIds_whenDown_empty(t *testing.T) {
	ps := longpoll.New()
	ps.MustSubscribe(time.Minute, "A")
	ps.Shutdown()
	if len(ps.Ids()) != 0 {
		t.Error("no ids expected")
	}
}

func TestLongPoll_onGet_success(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	id1 := ps.MustSubscribe(time.Minute, "A")
	id2 := ps.MustSubscribe(time.Minute, "B")
	time.Sleep(100 * time.Millisecond)
	datach1, _ := ps.Get(id1, 20*time.Second)
	datach2, _ := ps.Get(id2, 20*time.Second)
	time.Sleep(100 * time.Millisecond)
	ps.Publish(make(map[string]int), "A", "B")
	if len(<-datach1) != 1 {
		t.Error("expected 1 value on sub1")
	}
	if len(<-datach2) != 1 {
		t.Error("expected 1 value on sub2")
	}
}

func TestLongPoll_onGet_whenDown_error(t *testing.T) {
	ps := longpoll.New()
	id1 := ps.MustSubscribe(time.Minute, "A")
	time.Sleep(100 * time.Millisecond)
	ps.Shutdown()
	_, err := ps.Get(id1, 20*time.Second)
	if err == nil {
		t.Error("error expected")
	}
}

func TestLongPoll_onGet_wrongChannel_error(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	ps.MustSubscribe(time.Minute, "A")
	time.Sleep(100 * time.Millisecond)
	_, err := ps.Get("whatever", 20*time.Second)
	if err == nil {
		t.Error("error expected")
	}
}

func TestLongPoll_onGetThenDrop_empty(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	id := ps.MustSubscribe(time.Minute, "A")
	time.Sleep(100 * time.Millisecond)
	datach, _ := ps.Get(id, 20*time.Second)
	start := time.Now()
	ps.Drop(id)
	ps.Publish(make(map[string]int), "A")
	if len(<-datach) != 0 {
		t.Error("expected no data")
	}
	if time.Now().Sub(start) > time.Second {
		t.Error("get returned too late")
	}

}

func TestLongPoll_onDrop_success(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	id := ps.MustSubscribe(time.Minute, "A")
	if len(ps.Ids()) != 1 {
		t.Error("expected 1 channel")
	}
	ps.Drop(id)
	time.Sleep(100 * time.Millisecond)
	if len(ps.Ids()) > 0 {
		t.Error("expected no channels")
	}
}

func TestLongPoll_onDrop_whenDown_success(t *testing.T) {
	ps := longpoll.New()
	id := ps.MustSubscribe(time.Minute, "A")
	if len(ps.Ids()) != 1 {
		t.Error("expected 1 channel")
	}
	ps.Shutdown()
	time.Sleep(100 * time.Millisecond)
	ps.Drop(id) // no validations
}

func TestLongPoll_onDrop_wrongChannel_ignored(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	ps.Drop("foo") // no validations
}

func TestLongPoll_onShutdown_success(t *testing.T) {
	ps := longpoll.New()
	ps.MustSubscribe(time.Minute, "A")
	time.Sleep(100 * time.Millisecond)
	if len(ps.Ids()) != 1 {
		t.Error("expected 1 channel")
	}
	ps.Shutdown()
	time.Sleep(100 * time.Millisecond)
	if len(ps.Ids()) > 0 {
		t.Error("expected no channel")
	}
	if ps.IsAlive() {
		t.Error("expected pubsub down")
	}
	if err := ps.Publish(make(map[string]int), "A"); err == nil {
		t.Error("error expected")
	}
}

func TestLongPoll_onShutdown_whenDown_ignored(t *testing.T) {
	ps := longpoll.New()
	ps.Shutdown()
	ps.Shutdown()
	ps.Shutdown()
}

func TestLongPoll_onTopics_success(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	if len(ps.Topics()) > 0 {
		t.Error("no topics expected")
	}
	ps.MustSubscribe(time.Minute, "A", "B")
	ps.MustSubscribe(time.Minute, "B", "C")
	time.Sleep(100 * time.Millisecond)
	if len(ps.Topics()) != 3 {
		t.Error("3 expected")
	}
	ps.MustSubscribe(time.Minute, "C", "D")
	if len(ps.Topics()) != 4 {
		t.Error("4 expected")
	}
}

func TestLongPoll_onTopics_whenDown_empty(t *testing.T) {
	ps := longpoll.New()
	ps.MustSubscribe(time.Minute, "A", "B")
	ps.MustSubscribe(time.Minute, "B", "C")
	time.Sleep(100 * time.Millisecond)
	if len(ps.Topics()) != 3 {
		t.Error("3 topics expected")
	}
	ps.Shutdown()
	time.Sleep(100 * time.Millisecond)
	if len(ps.Topics()) > 0 {
		t.Error("no topics expected")
	}
}

func TestLongPoll_onTopics_uniqueAndSorted(t *testing.T) {
	ps := longpoll.New()
	defer ps.Shutdown()
	ps.MustSubscribe(time.Minute, "A", "B")
	ps.MustSubscribe(time.Minute, "B", "C")
	ps.MustSubscribe(time.Minute, "C", "D")
	time.Sleep(100 * time.Millisecond)
	if len(ps.Topics()) != 4 {
		t.Error("4 expected")
	}
	letters := [...]string{"A", "B", "C", "D"}
	for i := 0; i < 4; i++ {
		if letters[i] != ps.Topics()[i] {
			t.Errorf("wrong topic at index %v", i)
		}
	}
}
