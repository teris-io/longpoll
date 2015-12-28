// Copyright 2015 Ventu.io. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file

package longpoll_test

import (
	"github.com/op/go-logging"
	"os"
)

func testinit() {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	format := logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{longfunc}: %{level:.6s}%{color:reset} %{message}")
	logging.SetBackend(logging.NewBackendFormatter(backend, format))
}

var log = logging.MustGetLogger("test")
