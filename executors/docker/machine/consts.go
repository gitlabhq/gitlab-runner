package machine

import "time"

var provisionRetryInterval = time.Second
var machineDeadInterval = 20 * time.Minute
var removeRetryInterval = 30 * time.Second
var removeRetryTries = 3
var machineCanConnectCommandTimeout = 1 * time.Hour
var machineCreateCommandTimeout = 1 * time.Hour
var machineCredentialsCommandTimeout = 1 * time.Hour
var machineExistCommandTimeout = 1 * time.Hour
var machineRemoveCommandTimeout = 1 * time.Hour
var machineStopCommandTimeout = 1 * time.Minute
