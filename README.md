[![Build status][buildimage]][build] [![Coverage][codecovimage]][codecov] [![GoReportCard][cardimage]][card] [![API documentation][docsimage]][docs]

# Parked: PubSub with long polling in Go

The package `longpoll` provides an implementation of the
long-polling mechanism of the PubSub pattern. Although the primary purpose of the
library is to aid the development of web applications, the library provides no specific web
handlers and  can be used in other distributed applications.

Long polling is a technique to notify client applications about updates on the server. It is often
used in writing web application as a substitute for the push technique, however can be used in
other distributed applications.

Clients initiate subscriptions to the server specifying topics they are interested in. Given a
subscription Id a client makes a request for new data. The request is held open until data becomes
available on the server (published to a matching topic). As soon as this happens the request is
answered immediately. If no data arrives over a predefined time window (the long-polling interval)
the request returns empty. A new connection is then established between the client and the server
to receive further updates.

The following points are often listed as the benefits of long-polling over the push mechanism in web
applications:

* does not require a persistent connection to the server
* works for slow clients as they receive information at the speed they can process, although
maybe in large chunks which are accumulated at the server between requests
* friendly to proxies blocking streaming


## Implementation details and examples

Subscriptions will timeout and get closed if no client request is received over a given timeout
interval. Every request resets the timeout counter. The timeout interval is a property of the
subscription itself and different subscriptions may have different timeout intervals.

The long-polling interval, within which the request is held, is specified per request. Web
application wrappers might provide defaults.

The library supports concurrent long-polling requests on the same subscription Id, but no data will
be duplicated across request responses. No specific distribution of data across responses is
guaranteed: new requests signal the existing one to return immediately.

At the moment the library does not support persisting of published data before it is collected by
subscribers. All the published data is stored in memory of the backend.


**Long-polling with subscription management:**

Handling of long-polling subscriptions, publishing and receiving data is done by the
`longpoll.LongPoll` type:

```go
ps := longpoll.New()
id1, _ := ps.Subscribe(time.Minute, "TopicA", "TopicB")
id2, _ := ps.Subscribe(time.Minute, "TopicB", "TopicC")

go func() {
	for {
		if datach, err := ps.Get(id1, 30*time.Second); err == nil {
			fmt.Printf("%v received %v", id1, <-datach)
		} else {
			break
		}
	}
}()

go func() {
	for {
		if datach, err := ps.Get(id2, 20*time.Second); err == nil {
			fmt.Printf("%v received %v", id2, <-datach)
		} else {
			break
		}
	}
}()

for {
	// data published on TopicB will be received by id1 and id2, TopicC by id2 only
	ps.Publish({"random": rand.Float64()}, "TopicB", "TopicC")

	// sleep for up to 50s
	time.Sleep(time.Duration(rand.Intn(5e10)))
}
```

**Long-polling on a single subscription channel:**

Handling of the single-channel long-polling pubsub is done by the `longpoll.Sub` type:

```go
ch := longpoll.MustNewChannel(time.Minute, func (id string) {
	// action on exit
}, "TopicA", "TopicB")

go func() {
	for {
		if datach, err := ch.Get(20*time.Second); err == nil {
			fmt.Printf("received %v", <-datach)
		} else {
			break
		}
	}
}()

for {
	ch.Publish({"random": rand.Float64()}, "TopicB")
	// above subscription will not receive this data
	ch.Publish({"random": rand.Float64()}, "TopicC")

	// sleep for up to 50s
	time.Sleep(time.Duration(rand.Intn(5e10)))
}
```

### License and copyright

	Copyright (c) 2015-2017. Oleg Sklyar and teris.io. MIT license applies. All rights reserved.


[build]: https://travis-ci.org/teris-io/longpoll
[buildimage]: https://travis-ci.org/teris-io/longpoll.svg?branch=master

[codecov]: https://codecov.io/github/teris-io/longpoll?branch=master
[codecovimage]: https://codecov.io/github/teris-io/longpoll/coverage.svg?branch=master

[card]: http://goreportcard.com/report/teris-io/longpoll
[cardimage]: https://goreportcard.com/badge/github.com/teris-io/longpoll

[docs]: https://godoc.org/github.com/teris-io/longpoll
[docsimage]: http://img.shields.io/badge/godoc-reference-blue.svg?style=flat
