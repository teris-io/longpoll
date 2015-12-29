// Copyright 2015 Ventu.io. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file

package longpoll_test

import (
	"github.com/ventu-io/go-longpoll"
	"sort"
	"testing"
	"time"
)

func TestSub_onNoAction_SubExpiresOnTimeout(t *testing.T) {
	timeout := 400 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	var end time.Time

	sub := longpoll.NewSub(timeout, func(id string) {
		end = time.Now()
	}, "any")
	defer sub.Drop()

	if !sub.IsAlive() {
		t.Errorf("subscription not alive on start")
	}

	time.Sleep(timeout - tolerance)
	if !sub.IsAlive() {
		t.Errorf("subscription not alive before timeout")
	}

	time.Sleep(tolerance + tolerance)
	if sub.IsAlive() {
		t.Errorf("subscription alive on timeout")
	}
	if end.Sub(start) < timeout {
		t.Errorf("timeout too early")
	}
	if end.Sub(start) > timeout+tolerance {
		t.Errorf("timeout too late")
	}
}

func TestSub_onTimeout_handlerCalledWithCorrectId(t *testing.T) {
	timeout := 400 * time.Millisecond
	tolerance := 25 * time.Millisecond

	var idx string
	sub := longpoll.NewSub(timeout, func(id string) {
		idx = id
	}, "any")
	defer sub.Drop()

	time.Sleep(timeout + tolerance)
	if idx != sub.Id() {
		t.Errorf("no or incorrect subscription id in onClose handler")
	}
}

func TestSub_onNoHandler_successOnTimeout(t *testing.T) {
	timeout := 400 * time.Millisecond
	tolerance := 25 * time.Millisecond

	sub := longpoll.NewSub(timeout, nil, "any")
	defer sub.Drop()

	time.Sleep(timeout - tolerance)
	if !sub.IsAlive() {
		t.Errorf("subscription not alive before timeout")
	}

	time.Sleep(tolerance + tolerance)
	if sub.IsAlive() {
		t.Errorf("subscription alive on timeout")
	}
}

func TestSub_onNoPublish_GetExpires_andSubExpiresLater(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	sub := longpoll.NewSub(timeout, nil, "any")
	defer sub.Drop()

	time.Sleep(polltime)
	data := <-sub.Get(polltime)

	if time.Now().Sub(start) < polltime+polltime {
		t.Errorf("get returned before polltime")
	}
	if time.Now().Sub(start) > polltime+polltime+tolerance {
		t.Errorf("get returned way too late")
	}
	if len(data) > 0 {
		t.Errorf("unexpected data in get")
	}
	if !sub.IsAlive() {
		t.Errorf("subscription not upon get polltime exit")
	}

	time.Sleep(timeout - polltime + tolerance)
	if sub.IsAlive() {
		t.Errorf("subscription alive on timeout")
	}
}

func TestSub_onDrop_givenGetWaiting_properCleanup(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	sub := longpoll.NewSub(timeout, nil, "A", "B")
	if sub.QueueSize() != 0 {
		t.Errorf("unexpected queue size")
	}
	topics := sub.Topics()
	sort.Strings(topics)
	if topics[0] != "A" || topics[1] != "B" {
		t.Errorf("unexpected topics")
	}
	if sub.IsGetWaiting() {
		t.Errorf("unexpected get waiting")
	}

	datach := sub.Get(polltime)
	time.Sleep(tolerance)

	if !sub.IsGetWaiting() {
		t.Errorf("get not waiting")
	}

	sub.Drop()
	time.Sleep(tolerance)
	if len(<-datach) > 0 {
		t.Errorf("unexpected results in get")
	}
	if sub.IsGetWaiting() {
		t.Errorf("unexpected get waiting")
	}
	if len(sub.Topics()) > 0 {
		t.Errorf("no topics expected")
	}
	if sub.IsAlive() {
		t.Errorf("subscription unexpectedly alive")
	}
}

type pubdata struct {
	value int
}

func TestSub_onPublishThenGetThenGet_Get1ComesBackImmediately_Get2Expires(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	sub := longpoll.NewSub(timeout, nil, "A")
	defer sub.Drop()

	outdata := pubdata{value: 351}

	sub.Publish(&outdata, "A")
	time.Sleep(tolerance)
	data := <-sub.Get(polltime)

	if time.Now().Sub(start) > 2*tolerance {
		t.Errorf("get returned late")
	}
	if len(data) != 1 || data[0] != &outdata {
		t.Errorf("unexpected data in get")
	}
	if !sub.IsAlive() {
		t.Errorf("subscription not upon get polltime exit")
	}

	time.Sleep(polltime)
	data = <-sub.Get(polltime)
	if time.Now().Sub(start) < 2*polltime+tolerance {
		t.Errorf("get returned before polltime")
	}
	if time.Now().Sub(start) > 2*polltime+2*tolerance {
		t.Errorf("get returned way too late")
	}
	if len(data) > 0 {
		t.Errorf("unexpected data in get")
	}
	if !sub.IsAlive() {
		t.Errorf("subscription not upon get polltime exit")
	}
}

func TestSub_onGetThenPublish_GetComesBackUponPublish(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	sub := longpoll.NewSub(timeout, nil, "A")
	defer sub.Drop()

	datach := sub.Get(polltime)
	time.Sleep(polltime / 2)

	outdata := pubdata{value: 351}
	sub.Publish(&outdata, "A")
	time.Sleep(tolerance)

	data := <-datach
	if time.Now().Sub(start) > polltime/2+2*tolerance {
		t.Errorf("get returned late")
	}
	if len(data) != 1 || data[0] != &outdata {
		t.Errorf("unexpected data in get")
	}
	if !sub.IsAlive() {
		t.Errorf("subscription not upon get polltime exit")
	}
}

func TestSub_onGetThenGetThenPublish_Get1Expires_andGet2ComesWithData(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	sub := longpoll.NewSub(timeout, nil, "A")
	defer sub.Drop()

	datach1 := sub.Get(polltime)
	time.Sleep(polltime / 2)

	datach2 := sub.Get(polltime)
	time.Sleep(polltime / 2)

	data1 := <-datach1
	if time.Now().Sub(start) < polltime {
		t.Errorf("get1 returned early")
	}
	if time.Now().Sub(start) > polltime+tolerance {
		t.Errorf("get1 returned late")
	}
	if len(data1) > 0 {
		t.Errorf("unexpected data in get1")
	}

	outdata := pubdata{value: 351}
	sub.Publish(&outdata, "A")
	time.Sleep(tolerance)

	data2 := <-datach2

	if time.Now().Sub(start) > polltime+2*tolerance {
		t.Errorf("get2 returned late")
	}
	if len(data2) != 1 || data2[0] != &outdata {
		t.Errorf("unexpected data in get2")
	}
	if !sub.IsAlive() {
		t.Errorf("subscription not upon get polltime exit")
	}
}

func TestSub_onNxPublishThenGet_GetReceivesAll(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	sub := longpoll.NewSub(timeout, nil, "A", "C")
	defer sub.Drop()

	outdata1 := pubdata{value: 1}
	outdata2 := pubdata{value: 2}
	outdata3 := pubdata{value: 3}
	outdata4 := pubdata{value: 4}

	sub.Publish(&outdata1, "A")
	time.Sleep(polltime / 5)

	sub.Publish(&outdata2, "B")
	time.Sleep(polltime / 5)

	sub.Publish(&outdata3, "C")
	time.Sleep(polltime / 5)

	sub.Publish(&outdata4, "A")
	time.Sleep(polltime / 5)

	data := <-sub.Get(polltime)

	if time.Now().Sub(start) > 4*polltime/5+tolerance {
		t.Errorf("get returned late")
	}
	if len(data) != 3 || data[0] != &outdata1 || data[1] != &outdata3 || data[2] != &outdata4 {
		t.Errorf("unexpected data in get")
	}
	if !sub.IsAlive() {
		t.Errorf("subscription not upon get polltime exit")
	}
}

func TestSub_onPublish_withAnyMatchingTopic_GetReceives(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	sub := longpoll.NewSub(timeout, nil, "A", "B", "C", "D")
	defer sub.Drop()

	datach := sub.Get(polltime)
	time.Sleep(polltime / 2)

	outdata := pubdata{value: 351}
	sub.Publish(&outdata, "C")
	// sub.Publish(&outdata, "D") -- nondeterministic if get gets 1 or 2 due to concurrency
	time.Sleep(tolerance)

	data := <-datach
	if time.Now().Sub(start) > polltime/2+2*tolerance {
		t.Errorf("get returned late")
	}

	if len(data) != 1 || data[0] != &outdata {
		t.Errorf("unexpected data in get")
	}
	if !sub.IsAlive() {
		t.Errorf("subscription not upon get polltime exit")
	}
}

func TestSub_onPublish_withNonmatchingTopic_GetIndifferent(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	sub := longpoll.NewSub(timeout, nil, "A", "B", "C", "D")
	defer sub.Drop()

	datach := sub.Get(polltime)
	time.Sleep(polltime / 2)

	outdata := pubdata{value: 351}
	sub.Publish(&outdata, "Z")
	sub.Publish(&outdata, "25")
	sub.Publish(&outdata, "foo")
	sub.Publish(&outdata, "A")
	time.Sleep(tolerance)

	data := <-datach
	if time.Now().Sub(start) > polltime/2+2*tolerance {
		t.Errorf("get returned late")
	}
	if len(data) != 1 || data[0] != &outdata {
		t.Errorf("unexpected data in get")
	}
	if !sub.IsAlive() {
		t.Errorf("subscription not upon get polltime exit")
	}
}

func TestSub_onDroppedSub_callingGetOrPublishHasNoEffect(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	sub := longpoll.NewSub(timeout, nil, "A", "B", "C")

	outdata := pubdata{value: 351}
	sub.Publish(&outdata, "A")
	sub.Drop()
	sub.Publish(&outdata, "B")
	sub.Publish(&outdata, "C")
	time.Sleep(tolerance)

	data := <-sub.Get(polltime)
	if time.Now().Sub(start) > 2*tolerance {
		t.Errorf("get returned late")
	}
	if len(data) > 0 {
		t.Errorf("unexpected data in get")
	}
}

func TestSub_onDropRightAfterGet_GetReturnsEmpty(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	sub := longpoll.NewSub(timeout, nil, "A", "B", "C")
	datach := sub.Get(polltime)
	sub.Drop()
	if len(<-datach) > 0 {
		t.Errorf("data coming from nowhere")
	}
	if time.Now().Sub(start) > tolerance {
		t.Errorf("get returned late")
	}
}
