package machine

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/Sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
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
}

func (m *machineDetails) isUsed() bool {
	return m.State != machineStateIdle
}

func (m *machineDetails) isDead() bool {
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
		Details    machineDetails
		Time       string
		CreatedAgo time.Duration
	}
	details.Details = *m
	details.Time = time.Now().String()
	details.CreatedAgo = time.Since(m.Created)
	data := helpers.ToYAML(&details)
	ioutil.WriteFile("machines/"+details.Details.Name+".yml", []byte(data), 0600)
}

type machinesDetails map[string]*machineDetails
