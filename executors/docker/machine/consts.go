package machine

import "time"

var provisionRetryInterval = time.Second
var machineDeadInterval = 20 * time.Minute
var removeRetryInterval = 30 * time.Second
var removeRetryTries = 3
var machineStopCommandTimeout = 1 * time.Minute
