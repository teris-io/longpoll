### Work in progress...

Before first (minor) release:

* `longpoll.Sub` is complete and fully tested.
* `longpoll.LongPoll` is feature complete, but the API, in particular error handling, is subject
to change.

# Go long-polling library

[![godoc][godocimage]][godocimage]


The [Go][go] library `go-longpoll` (package `longpoll`) provides an implementation of the
long-polling mechanism of the [PubSub][pubsub] pattern. Although the primary purpose of the
library is to aid the development of web applications, the library provides no specific web
handlers and  can be used in other distributed applications.

Long-polling is a technique to notify client applications about updates on the server. It is often
used in writing web application as a substitute for the push technique, however can be used in
other distributed applications.

Clients initiate subscriptions to the server specifying topics they are interested in. Given a
subscription Id a client makes a request for new data. The request connection is held open until
data becomes available on the server (published to a matching topic). When data becomes available
the request is answered immediately and the connection closes. If this does not happen over a
predefined time window (the long-polling interval) the connection closes returning no data. A new
connection is then established between the client and the server to receive further updates.

The following points are often listed as the benefits of long-polling over the push mechanism in web
applications:

* does not require a persistent connection to the server
* works for slow clients as they receive information at the speed they can process, although
maybe in large chunks which are accumulated at the server between requests
* friendly to proxies blocking streaming

The library can be installed by one of the following methods:

* using `go get`

	```
	go get github.com/ventu-io/go-longpoll
	```

* via cloning this repository:

	```
	git clone git@github.com:ventu-io/go-longpoll.git ${GOPATH}/src/github.com/ventu-io/go-longpoll
	```

## Implementation details and examples

The API documentation is available at [GoDocs][docs]. The following demo [repository][demo]
provides multiple examples and benchmarks for this library.

Subscriptions will timeout and get closed if no client request is received over a given timeout
interval. Every request resets the timeout counter. The timeout interval is a property of the
subscription itself and different subscriptions may have different timeout intervals.

The long-polling interval within which the request stays open is specified per request. Web
application wrappers might provide defaults.

The library supports concurrent long-polling requests on the same subscription Id, but no data will
be duplicated across request responses. No specific distribution of data across such requests is not
guaranteed: new requests signal the existing one to return immediately.

At the moment the library does not support persisting of published data before it is collected by
subscribers. All the published data is stored in memory of the backend.


**Long-polling with subscription management:**

Handling of long-polling subscriptions, publishing and receiving data is done by the
`longpoll.LongPoll` type:

```go
ps := longpoll.New()
id1 := ps.Subscribe(time.Minute, "TopicA", "TopicB")
id2 := ps.Subscribe(time.Minute, "TopicB", "TopicC")

go func() {
	for {
		ch, ok := ps.Get(id1, 30*time.Second)
		fmt.Printf("%v received %v", id1, <-ch)
	}
}()

go func() {
	for {
		ch, ok := ps.Get(id2, 30*time.Second)
		fmt.Printf("%v received %v", id2, <-ch)
	}
}()

go func() {
	for {
		// data published on TopicB will be received by id1 and id2, TopicC by id2 only
		ps.Publish({"random": rand.Float64()}, "TopicB", "TopicC")

		// sleep for up to 50s
		time.Sleep(time.Duration(rand.Intn(5e10)))
	}
}()
```
A comprehensive example can be run from the demo [repository][demo] via

    go build
    ./go-pubsub-examples 1

**Long-polling on a single subscription channel:**

Handling of single-channel long-polling pubsub is done by the `longpoll.Sub` type:

```go
sub := longpoll.NewSub(time.Minute, func (id string) {
	// action on exit
}, "TopicA", "TopicB")

go func() {
	for {
		fmt.Printf("received %v", <-sub.Get(30*time.Second))
	}
}()

go func() {
	for {
		sub.Publish({"random": rand.Float64()}, "TopicB")
		// above subscription will not receive this data
		sub.Publish({"random": rand.Float64()}, "TopicC")

		// sleep for up to 50s
		time.Sleep(time.Duration(rand.Intn(5e10)))
	}
}()
```
A comprehensive example can be run from the demo [repository][demo] via

    go build
    ./go-pubsub-examples 2

### Performance

Using the code from the demo [repository][demo] a throughput test can be run using the command
below. Measured on conventional hardware long-polling on a single channel processes 1 million data
samples in (0.8, 2.5) seconds. The time is measured from the moment the first sample is published
till the last one is received.

    go build
    ./go-pubsub-examples 3
    ./go-pubsub-examples 4

## License

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the [license conditions][license] are met.

[go]: https://golang.org
[godocimage]: http://img.shields.io/badge/godoc-reference-blue.svg?style=flat
[pubsub]: https://en.wikipedia.org/wiki/Publish–subscribe_pattern
[docs]: https://godoc.org/github.com/ventu-io/go-longpoll
[demo]:    https://github.com/ventu-io/go-pubsub-examples/
[license]: https://github.com/ventu-io/go-longpoll/blob/master/LICENSE
