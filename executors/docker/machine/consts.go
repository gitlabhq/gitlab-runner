package machine

import "time"

var provisionRetryInterval = time.Second
var removeRetryInterval = 30 * time.Second
var removeRetryTries = 3
var shutdownDrainPollInterval = 10 * time.Second

var defaultMaxRemovalAttempts = 30
var machineCanConnectCommandTimeout = 1 * time.Hour
var machineCreateCommandTimeout = 1 * time.Hour
var machineCredentialsCommandTimeout = 1 * time.Hour
var machineExistCommandTimeout = 1 * time.Hour
var machineRemoveCommandTimeout = 1 * time.Hour
var machineStopCommandTimeout = 1 * time.Minute

// Consecutive Acquire passes an idle machine may be missing from `docker-machine ls` before it is dropped.
const vanishedAfterMisses = 3
