package machine

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

type machineDetails struct {
	Name       string
	Created    time.Time `yaml:"-"`
	Used       time.Time `yaml:"-"`
	UsedCount  int
	State      machineState
	Reason     string
	RetryCount int
	LastSeen   time.Time

	// targets caches target_* metric labels for this machine. Populated
	// from docker-machine config.json via targetLabelsFromInspect in
	// ensureDetails (on first observation) and overwritten in
	// createWithGrowthCapacity (post-Create, to catch driver-side
	// mutations like regional MIG's zone discovery).
	targets targetLabels `yaml:"-"`

	// maxRemovalAttempts copies config.Machine.MaxRemovalAttempts at create
	// time so finalizeRemoval doesn't need the runner config plumbed in.
	// Zero means the field wasn't set (e.g. machine recovered from on-disk
	// state across a runner-manager restart); finalizeRemoval falls back to
	// defaultMaxRemovalAttempts in that case.
	maxRemovalAttempts int `yaml:"-"`

	// inFlight is closed when the async create/remove goroutine mutating
	// this machine finishes. create() and remove() set it before spawning
	// the goroutine. A new operation replaces it and it is never reset to
	// nil, so a waiter must re-read it after each wake to follow a
	// create-then-remove handoff.
	inFlight chan struct{}

	// removalGaveUp is set when finalizeRemoval exhausts its retries and
	// leaves the remote VM possibly orphaned. A concurrent drain reads it
	// to tell an abandoned removal apart from a successful one.
	removalGaveUp bool `yaml:"-"`

	lock sync.Mutex
}

type machineInfo struct {
	Name       string
	Created    time.Time `yaml:"-"`
	Used       time.Time `yaml:"-"`
	UsedCount  int
	State      machineState
	Reason     string
	RetryCount int
	LastSeen   time.Time
}

func (m *machineDetails) Lock() {
	m.lock.Lock()
}

func (m *machineDetails) Unlock() {
	m.lock.Unlock()
}

func (m *machineDetails) info() machineInfo {
	return machineInfo{
		Name:       m.Name,
		Created:    m.Created,
		Used:       m.Used,
		UsedCount:  m.UsedCount,
		State:      m.State,
		Reason:     m.Reason,
		RetryCount: m.RetryCount,
		LastSeen:   m.LastSeen,
	}
}

func (m *machineDetails) isPersistedOnDisk() bool {
	// Machines in creating phase might or might not be persisted on disk
	// this is due to async nature of machine creation process
	// where to `docker-machine create` is the one that is creating relevant files
	// and it is being executed with undefined delay
	return m.State != machineStateCreating
}

func (m *machineDetails) isUsed() bool {
	return m.State != machineStateIdle
}

func (m machineInfo) isStuckOnRemove() bool {
	return m.State == machineStateRemoving && m.RetryCount >= removeRetryTries
}

func (m machineInfo) isDead() bool {
	return m.State == machineStateIdle &&
		time.Since(m.LastSeen) > machineDeadInterval
}

func (m *machineDetails) canBeUsed() bool {
	return m.State == machineStateAcquired
}

func (m *machineDetails) match(machineFilter string) bool {
	var query string
	if n, _ := fmt.Sscanf(m.Name, machineFilter, &query); n != 1 {
		return false
	}
	return true
}

func (m *machineDetails) writeDebugInformation() {
	if logrus.GetLevel() < logrus.DebugLevel {
		return
	}

	var details struct {
		Details    machineInfo
		Time       string
		CreatedAgo time.Duration
	}

	details.Details = m.info()
	details.Time = time.Now().String()
	details.CreatedAgo = time.Since(m.Created)
	data := helpers.ToYAML(&details)
	_ = os.WriteFile("machines/"+details.Details.Name+".yml", []byte(data), 0o600)
}

func (m *machineDetails) logger() *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"name":      m.Name,
		"lifetime":  time.Since(m.Created),
		"used":      time.Since(m.Used),
		"usedCount": m.UsedCount,
		"reason":    m.Reason,
	})
}

type machinesDetails map[string]*machineDetails
