// Copyright 2015 Ventu.io. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file

package examples

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"ventu.tech/ventu-io/pubsub/lpoll"
)

const pubcount = 1e6

// Benchmarking the throughput of pub/sub using consequent gets (new get starts only
// when the current one finishes).
func TestSubThroughput_onConsequentGets_expectFewEmptyGetsAndLarge100kChunks(t *testing.T) {

	t.Skip("The performance is dependant on the speed of the CPU: unskip to benchmark.")

	timeout := 10 * time.Second
	polltime := 100 * time.Millisecond

	subdone := make(chan bool, 1)
	pubdone := make(chan bool, 1)
	gethist := make(map[int]int)
	var received int32 = 0
	start := time.Now()

	sub := lpoll.NewSub(timeout, polltime, nil, "A")
	defer sub.Drop()

	var mx sync.Mutex
	go startget(sub, mx, subdone, gethist, &received)
	go startpub(sub, pubdone)
	whenDone(t, start, pubdone, subdone, gethist, &received)
	if count := gethist[0]; count > 1 {
		t.Errorf("unexpected empty gets (%v > 1) in sequential execution", count)
	}
	if count := gethist[100000]; count < 2 {
		t.Errorf("unexpectedly few (%v < 2) large 100k chunks in sequential execution", count)
	}
}

// Benchmarking the throughput of pub/sub using concurrent gets from three sources (new get
// may start when another one already running). If the current get waits for data, it will
// quit empty when new one arrives. In total, however, all the data must be received.
func TestSubThroughput_onConcurrentGets_expectEmptyGetsAndSmaller10kChunks(t *testing.T) {

	t.Skip("The performance is dependant on the speed of the CPU: unskip to benchmark.")

	timeout := 10 * time.Second
	polltime := 100 * time.Millisecond

	subdone := make(chan bool, 1)
	pubdone := make(chan bool, 1)
	gethist := make(map[int]int)
	var received int32 = 0
	start := time.Now()

	sub := lpoll.NewSub(timeout, polltime, nil, "A")
	defer sub.Drop()

	var mx sync.Mutex
	go startget(sub, mx, subdone, gethist, &received)
	go startget(sub, mx, subdone, gethist, &received)
	go startget(sub, mx, subdone, gethist, &received)
	go startpub(sub, pubdone)
	whenDone(t, start, pubdone, subdone, gethist, &received)
	if count := gethist[0]; count < 10 {
		t.Errorf("unexpectedly few (%v < 10) empty gets in concurrent execution", count)
	}
	if count := gethist[100000]; count > 2 {
		t.Errorf("unexpected large 100k chunks (%v > 2) in concurrent execution", count)
	}
}

func startget(sub *lpoll.Sub, mx sync.Mutex, subdone chan bool, gethist map[int]int, received *int32) {

	for {
		gotsofar := atomic.LoadInt32(received)
		if gotsofar >= pubcount || !sub.IsAlive() {
			break
		}
		getlen := len(<-sub.Get())
		mx.Lock()
		*received += int32(getlen)

		key := int(math.Pow10(int(math.Floor(0.5 + math.Log10(float64(getlen))))))
		if count, ok := gethist[key]; ok {
			gethist[key] = count + 1
		} else {
			gethist[key] = 1
		}
		mx.Unlock()
	}
	subdone <- true
}

func startpub(sub *lpoll.Sub, pubdone chan bool) {
	pubdata := struct {
		value int
	}{
		value: 25,
	}

	for i := 0; i < pubcount; i++ {
		sub.Publish(&pubdata, "A")
	}
	pubdone <- true
}

func whenDone(t *testing.T, start time.Time, pubdone chan bool, subdone chan bool, gethist map[int]int, received *int32) {
	<-pubdone
	pubstopped := time.Now()
	log.Infof("published %v in %v", pubcount, pubstopped.Sub(start))
	log.Noticef("received %v%%", 100*(*received)/pubcount)

	<-subdone
	substopped := time.Now()
	log.Infof("received %v in %v", *received, substopped.Sub(start))
	log.Noticef("received remainder in %v", substopped.Sub(pubstopped))

	gettotal := 0
	var keys []int
	for key, count := range gethist {
		gettotal += count
		keys = append(keys, key)
	}
	log.Infof("executed %v Get requests", gettotal)

	sort.Ints(keys)
	for _, key := range keys {
		log.Infof("received ~%v items %v times", key, gethist[key])
	}

	if *received != pubcount {
		t.Errorf("received %v instead of published %v", *received, pubcount)
	}
}
