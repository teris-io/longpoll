package longpoll

import (
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("longpoll")

const (
	no int32 = iota
	yes
)
