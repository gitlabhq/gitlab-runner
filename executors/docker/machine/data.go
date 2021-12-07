package machine

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type machinesData struct {
	Runner          string
	Acquired        int
	Creating        int
	Idle            int
	Used            int
	Removing        int
	StuckOnRemoving int
}

func (d *machinesData) InUse() int {
	return d.Used
}

func (d *machinesData) Available() int {
	return d.Acquired + d.Creating + d.Idle
}

func (d *machinesData) Total() int {
	return d.Acquired + d.Creating + d.Idle + d.Used + d.Removing + d.StuckOnRemoving
}

func (d *machinesData) Add(details *machineDetails) {
	switch details.State {
	case machineStateIdle:
		d.Idle++

	case machineStateCreating:
		d.Creating++

	case machineStateAcquired:
		d.Acquired++

	case machineStateUsed:
		d.Used++

	case machineStateRemoving:
		if details.isStuckOnRemove() {
			d.StuckOnRemoving++
		} else {
			d.Removing++
		}
	}
}

func (d *machinesData) Fields() logrus.Fields {
	return logrus.Fields{
		"runner":   d.Runner,
		"used":     d.Used,
		"idle":     d.Idle,
		"total":    d.Total(),
		"creating": d.Creating,
		"removing": d.Removing,
	}
}

func (d *machinesData) writeDebugInformation() {
	if logrus.GetLevel() < logrus.DebugLevel {
		return
	}

	file, err := os.OpenFile("machines.csv", os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()
	_, _ = fmt.Fprintln(
		file,
		"time", time.Now(),
		"runner", d.Runner,
		"acquired", d.Acquired,
		"creating", d.Creating,
		"idle", d.Idle,
		"used", d.Used,
		"removing", d.Removing,
	)
}
