# go-pubsub

`go-pubsub` is a Go library providing implementations of the Publish/Subscribe (pubsub) messaging paradigm.
Currently the library supports ~~basic local messaging and~~ long polling subscribers for web applications.

The documentation is available at [GoDocs][docs].

The library can be installed using one of the following two methods:

* by using the `go get`:

      go get github.com/ventu-io/go-pubsub

* by cloning this repository:

      git clone git@github.com:ventu-io/go-pubsub.git ${GOPATH}/src/github.com/ventu-io/go-pubsub

## Long polling example

The `longpoll` package provides a pubsub implementation for long polling subscribers to be used in web
servers or other distributed systems.

When using plain polling, clients send poll requests to the server asking for new data or events, which return
immediately with or without data, depending on the data availability on the server. In long polling poll requests
return as soon as data are available, but wait for data if required over a long polling interval.
If no data becomes available before the end of this interval the request returns with no data.

The requests are normally immediately resubmitted to provide continuous messaging and to guarantee that the
subscription does not timeout. The library supports concurrent long polling requests on the same subscription Id,
but no data will be duplicated across request responses. The distribution of data across such requests is
not guaranteed because new requests signal the existing one to return (empty).

The library does not provide any web server handlers, but only the backend allowing to implement simple wrappers
for integrating into a web server. No data persistence is currently provided nor  backends supported (i.e. running
down the backend may lead to data loss). This might change in future releases.

The following [repository][demo] provides multiple examples for a single channel/subscription operation
and for a full-stack mult-subscription long polling pubsub.

**Long polling on a single subscription channel:**

Running the example:

    cd ${GOPATH}/src/github.com/ventu-io/go-pubsub && go build && ./demo longpoll-single

The API to implement long polling for a single subscription channel or single client involves the following elements:

* `longpoll.Sub`: the type representing a subscription and providing methods to publish and receive data;
* `sub := longpoll.NewSub(timeout, exitHandler, topics...)`: creates a subscription,  where `exitHandler` may be `nil`;
* `sub.Publish(data, topic)`: non-blocking publishing;
* `data := <-sub.Get(pollDuration)`: receives data within `pollDuration`.

**Long polling with subscription management:**

Running the example:

    cd ${GOPATH}/src/github.com/ventu-io/go-pubsub && go build && ./demo longpoll

The API to implement long polling with subscription management involves the following elements:

* `longpoll.LongPoll`: the type representing the exchange and providing methods to subscribe, publish and receive data
as well as subscription management;
* `ps := longpoll.New()`: creates a new exchange;
* `id := ps.Subscribe(timeout, topics...)`
* `ps.Publish(data, topics...)`: non-blocking publishing to all subscriptions of matching topics;
* `ch, ok := ps.Get(id, pollDuration)` and `data := <-ch`: receiving data within `pollDuration` for subscription with Id `id`.


## License

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the [license conditions][license] are met.


[docs]: http://godoc.org/github.com/ventu-io/go-pubsub
[license]: https://github.com/ventu-io/go-pubsub/blob/master/LICENSE
[demo]:    https://github.com/ventu-io/go-pubsub-examples/
