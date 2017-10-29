// Copyright (c) 2015-2017. Oleg Sklyar & teris.io. All rights reserved.
// See the LICENSE file in the project root for licensing information.

package longpoll_test

import (
	"sort"
	"testing"
	"time"

	"github.com/teris-io/longpoll"
)

func TestChannel_onNewChannel_active(t *testing.T) {
	timeout := 400 * time.Millisecond

	ch, err := longpoll.NewChannel(timeout, func(id string) {}, "any")
	if err != nil {
		t.Error(err)
	}
	if !ch.IsAlive() {
		t.Error("channel down on construction")
	}
	if len(ch.ID()) < 9 || len(ch.ID()) > 11 {
		t.Error("expected an Id from go-shortid")
	}
}

func TestChannel_onMustNewChannel_active(t *testing.T) {
	timeout := 400 * time.Millisecond

	ch := longpoll.MustNewChannel(timeout, func(id string) {}, "any")
	if !ch.IsAlive() {
		t.Error("channel down on construction")
	}
}

func TestChannel_onNewChannel_withZeroTimeout_error(t *testing.T) {
	_, err := longpoll.NewChannel(0*time.Second, nil, "any")
	if err == nil {
		t.Errorf("error expected")
	}
}

func TestChannel_onNewChannel_withEmptyTopics_error(t *testing.T) {
	var topics []string
	_, err := longpoll.NewChannel(10*time.Second, nil, topics...)
	if err == nil {
		t.Errorf("error expected")
	}
}

func TestChannel_onMustNewChannel_withEmptyTopics_panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("panic expected")
		}
	}()
	var topics []string
	longpoll.MustNewChannel(10*time.Second, nil, topics...)
}

func TestChannel_onDuplicateTopics_uniqued(t *testing.T) {
	timeout := 400 * time.Millisecond

	ch := longpoll.MustNewChannel(timeout, func(id string) {}, "A", "B", "C", "B", "A")
	if len(ch.Topics()) != 3 {
		t.Error("wrong topics")
	}
}

func TestChannel_onNewChannel_andNoAction_expires(t *testing.T) {
	timeout := 400 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	var end time.Time

	ch := longpoll.MustNewChannel(timeout, func(id string) {
		end = time.Now()
	}, "any")
	defer ch.Drop()

	if !ch.IsAlive() {
		t.Errorf("channel not alive on start")
	}

	time.Sleep(timeout - tolerance)
	if !ch.IsAlive() {
		t.Errorf("channel not alive before timeout")
	}

	time.Sleep(tolerance + tolerance)
	if ch.IsAlive() {
		t.Errorf("channel alive on timeout")
	}
	if end.Sub(start) < timeout {
		t.Errorf("timeout too early")
	}
	if end.Sub(start) > timeout+tolerance {
		t.Errorf("timeout too late")
	}
}

func TestChannel_onTimeout_handlerCalledWithCorrectId(t *testing.T) {
	timeout := 400 * time.Millisecond
	tolerance := 25 * time.Millisecond

	var idx string
	ch := longpoll.MustNewChannel(timeout, func(id string) {
		idx = id
	}, "any")
	defer ch.Drop()

	time.Sleep(timeout + tolerance)
	if idx != ch.ID() {
		t.Errorf("no or incorrect channel id in onClose handler")
	}
}

func TestChannel_onNoHandler_whenTimeout_success(t *testing.T) {
	timeout := 400 * time.Millisecond
	tolerance := 25 * time.Millisecond

	ch := longpoll.MustNewChannel(timeout, nil, "any")
	defer ch.Drop()

	time.Sleep(timeout - tolerance)
	if !ch.IsAlive() {
		t.Errorf("channel not alive before timeout")
	}

	time.Sleep(tolerance + tolerance)
	if ch.IsAlive() {
		t.Errorf("channel alive on timeout")
	}
}

func TestChannel_onGetAndNoPublish_expiresGetAndChannel(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	ch := longpoll.MustNewChannel(timeout, nil, "any")
	defer ch.Drop()

	time.Sleep(polltime)
	datach, _ := ch.Get(polltime)
	data := <-datach

	if time.Now().Sub(start) < polltime+polltime {
		t.Errorf("get returned before polltime")
	}
	if time.Now().Sub(start) > polltime+polltime+tolerance {
		t.Errorf("get returned way too late")
	}
	if len(data) > 0 {
		t.Errorf("unexpected data in get")
	}
	if !ch.IsAlive() {
		t.Errorf("channel not upon get polltime exit")
	}

	time.Sleep(timeout - polltime + tolerance)
	if ch.IsAlive() {
		t.Errorf("channel alive on timeout")
	}
}

func TestChannel_onGet_withZeroPollTime_error(t *testing.T) {
	timeout := 400 * time.Millisecond

	ch := longpoll.MustNewChannel(timeout, nil, "any")
	defer ch.Drop()

	_, err := ch.Get(0 * time.Millisecond)
	if err == nil {
		t.Error("error expected")
	}
}

func TestChannel_onDrop_withGetWaiting_cleanup(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	ch := longpoll.MustNewChannel(timeout, nil, "A", "B")
	if ch.QueueSize() != 0 {
		t.Errorf("unexpected queue size")
	}
	topics := ch.Topics()
	sort.Strings(topics)
	if topics[0] != "A" || topics[1] != "B" {
		t.Errorf("unexpected topics")
	}
	if ch.IsGetWaiting() {
		t.Errorf("unexpected get waiting")
	}

	datach, _ := ch.Get(polltime)
	time.Sleep(tolerance)

	if !ch.IsGetWaiting() {
		t.Errorf("get not waiting")
	}

	ch.Drop()
	time.Sleep(tolerance)
	if len(<-datach) > 0 {
		t.Errorf("unexpected results in get")
	}
	if ch.IsGetWaiting() {
		t.Errorf("unexpected get waiting")
	}
	if len(ch.Topics()) != 2 {
		t.Errorf("expected unchanged topics")
	}
	if ch.IsAlive() {
		t.Errorf("channel unexpectedly alive")
	}
}

type pubdata struct {
	value int
}

func TestChannel_onPublishThenGetThenGet_Get1ComesBackImmediately_Get2Expires(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	ch := longpoll.MustNewChannel(timeout, nil, "A")
	defer ch.Drop()

	outdata := pubdata{value: 351}

	ch.Publish(&outdata, "A")
	time.Sleep(tolerance)
	datach, _ := ch.Get(polltime)
	data := <-datach

	if time.Now().Sub(start) > 2*tolerance {
		t.Errorf("get returned late")
	}
	if len(data) != 1 || data[0] != &outdata {
		t.Errorf("unexpected data in get")
	}
	if !ch.IsAlive() {
		t.Errorf("channel not upon get polltime exit")
	}

	time.Sleep(polltime)
	datach, _ = ch.Get(polltime)
	data = <-datach
	if time.Now().Sub(start) < 2*polltime+tolerance {
		t.Errorf("get returned before polltime")
	}
	if time.Now().Sub(start) > 2*polltime+2*tolerance {
		t.Errorf("get returned way too late")
	}
	if len(data) > 0 {
		t.Errorf("unexpected data in get")
	}
	if !ch.IsAlive() {
		t.Errorf("channel not upon get polltime exit")
	}
}

func TestChannel_onGetThenPublish_GetComesBackUponPublish(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	ch := longpoll.MustNewChannel(timeout, nil, "A")
	defer ch.Drop()

	datach, _ := ch.Get(polltime)
	time.Sleep(polltime / 2)

	outdata := pubdata{value: 351}
	ch.Publish(&outdata, "A")
	time.Sleep(tolerance)

	data := <-datach
	if time.Now().Sub(start) > polltime/2+2*tolerance {
		t.Errorf("get returned late")
	}
	if len(data) != 1 || data[0] != &outdata {
		t.Errorf("unexpected data in get")
	}
	if !ch.IsAlive() {
		t.Errorf("channel not upon get polltime exit")
	}
}

func TestChannel_onGetThenGetThenPublish_Get1Expires_andGet2ComesWithData(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	ch := longpoll.MustNewChannel(timeout, nil, "A")
	defer ch.Drop()

	datach1, _ := ch.Get(polltime)
	time.Sleep(polltime / 2)

	datach2, _ := ch.Get(polltime)
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
	ch.Publish(&outdata, "A")
	time.Sleep(tolerance)

	data2 := <-datach2

	if time.Now().Sub(start) > polltime+2*tolerance {
		t.Errorf("get2 returned late")
	}
	if len(data2) != 1 || data2[0] != &outdata {
		t.Errorf("unexpected data in get2")
	}
	if !ch.IsAlive() {
		t.Errorf("channel not upon get polltime exit")
	}
}

func TestChannel_onNxPublishThenGet_GetReceivesAll(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	ch := longpoll.MustNewChannel(timeout, nil, "A", "C")
	defer ch.Drop()

	outdata1 := pubdata{value: 1}
	outdata2 := pubdata{value: 2}
	outdata3 := pubdata{value: 3}
	outdata4 := pubdata{value: 4}

	ch.Publish(&outdata1, "A")
	time.Sleep(polltime / 5)

	ch.Publish(&outdata2, "B")
	time.Sleep(polltime / 5)

	ch.Publish(&outdata3, "C")
	time.Sleep(polltime / 5)

	ch.Publish(&outdata4, "A")
	time.Sleep(polltime / 5)

	datach, _ := ch.Get(polltime)
	data := <-datach

	if time.Now().Sub(start) > 4*polltime/5+tolerance {
		t.Errorf("get returned late")
	}
	if len(data) != 3 || data[0] != &outdata1 || data[1] != &outdata3 || data[2] != &outdata4 {
		t.Errorf("unexpected data in get")
	}
	if !ch.IsAlive() {
		t.Errorf("channel not upon get polltime exit")
	}
}

func TestChannel_onPublish_withAnyMatchingTopic_GetReceives(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	ch := longpoll.MustNewChannel(timeout, nil, "A", "B", "C", "D")
	defer ch.Drop()

	datach, _ := ch.Get(polltime)
	time.Sleep(polltime / 2)

	outdata := pubdata{value: 351}
	ch.Publish(&outdata, "C")
	// ch.Publish(&outdata, "D") -- nondeterministic if get gets 1 or 2 due to concurrency
	time.Sleep(tolerance)

	data := <-datach
	if time.Now().Sub(start) > polltime/2+2*tolerance {
		t.Errorf("get returned late")
	}

	if len(data) != 1 || data[0] != &outdata {
		t.Errorf("unexpected data in get")
	}
	if !ch.IsAlive() {
		t.Errorf("channel not upon get polltime exit")
	}
}

func TestChannel_onPublish_withNonmatchingTopic_GetIndifferent(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	ch := longpoll.MustNewChannel(timeout, nil, "A", "B", "C", "D")
	defer ch.Drop()

	datach, _ := ch.Get(polltime)
	time.Sleep(polltime / 2)

	outdata := pubdata{value: 351}
	ch.Publish(&outdata, "Z")
	ch.Publish(&outdata, "25")
	ch.Publish(&outdata, "foo")
	ch.Publish(&outdata, "A")
	time.Sleep(tolerance)

	data := <-datach
	if time.Now().Sub(start) > polltime/2+2*tolerance {
		t.Errorf("get returned late")
	}
	if len(data) != 1 || data[0] != &outdata {
		t.Errorf("unexpected data in get")
	}
	if !ch.IsAlive() {
		t.Errorf("channel not upon get polltime exit")
	}
}

func TestChannel_onDroppedSub_GetErrors(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	ch := longpoll.MustNewChannel(timeout, nil, "A", "B", "C")

	outdata := pubdata{value: 351}
	ch.Publish(&outdata, "A")
	ch.Drop()
	time.Sleep(tolerance)

	_, err := ch.Get(polltime)
	if time.Now().Sub(start) > 2*tolerance {
		t.Errorf("get returned late")
	}
	if err == nil {
		t.Errorf("error expected")
	}
}

func TestChannel_onDroppedSub_PublishErrors(t *testing.T) {
	timeout := 400 * time.Millisecond
	tolerance := 25 * time.Millisecond

	ch := longpoll.MustNewChannel(timeout, nil, "A", "B", "C")

	outdata := pubdata{value: 351}
	ch.Publish(&outdata, "A")
	ch.Drop()
	time.Sleep(tolerance)

	err := ch.Publish(&outdata, "B")

	if err == nil {
		t.Errorf("error expected")
	}
}

func TestChannel_onDropRightAfterGet_GetReturnsEmpty(t *testing.T) {
	timeout := 400 * time.Millisecond
	polltime := 200 * time.Millisecond
	tolerance := 25 * time.Millisecond

	start := time.Now()
	ch := longpoll.MustNewChannel(timeout, nil, "A", "B", "C")
	datach, _ := ch.Get(polltime)
	ch.Drop()
	if len(<-datach) > 0 {
		t.Errorf("data coming from nowhere")
	}
	if time.Now().Sub(start) > tolerance {
		t.Errorf("get returned late")
	}
}
