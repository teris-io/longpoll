package longpoll

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("lpoll")

const (
	no int32 = iota
	yes
)

const Version = 1.0
