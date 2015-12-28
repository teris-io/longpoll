// Copyright 2015 Ventu.io. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file

package longpoll_test

import (
	"testing"
	"time"
	lp "ventu.tech/ventu-io/longpoll"
)

func TestGet_whenNewSub_thenPolltimeAndTimeout(t *testing.T) {
	testinit()

	start := time.Now()
	var end time.Time

	sub := lp.NewSubscription(500*time.Millisecond, 250*time.Millisecond, func(id string) {
		end = time.Now()
	}, "any")
	if !sub.IsAlive() {
		t.Errorf("subscription not alive")
	}
	time.Sleep(600 * time.Millisecond)
	if sub.IsAlive() {
		t.Errorf("subscription is alive after timeout")
	}
	if end.Sub(start) < 500*time.Millisecond {
		t.Errorf("timeout too early")
	}
}
