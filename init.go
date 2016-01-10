// Copyright (c) 2015 Ventu.io, Oleg Sklyar, contributors
// The use of this source code is governed by a MIT style license found in the LICENSE file

package longpoll

import "github.com/ventu-io/go-log-interface"

const (
	no int32 = iota
	yes
)

// Version of the library.
const Version = 1.1

func logger() log.Logger {
	return log.GetLogger("longpoll")
}
