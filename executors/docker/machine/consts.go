package machine

import "time"

var provisionRetryInterval = time.Second
var removalRetryInterval = time.Minute
var useMachineRetries = 10
var useMachineRetryInterval = 10 * time.Second
