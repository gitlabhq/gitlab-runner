package stats_server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

func TestStatsDataJSONEncoding(t *testing.T) {
	startedAt, _ := time.Parse(time.RFC3339, "2016-07-02T00:01:44+02:00")
	configReloadedAt, _ := time.Parse(time.RFC3339, "2016-07-02T00:01:44+02:00")
	buildAt, _ := time.Parse(time.RFC3339, "2016-07-02T00:01:28+02:00")

	data := StatsData{
		StartedAt:        startedAt,
		ConfigReloadedAt: configReloadedAt,
		BuildsCount:      5,
		RunnersBuildsCounts: map[string]int{
			"abc1234": 3,
			"def5678": 2,
		},
		Uptime: 0.1234,
		VersionInfo: common.AppVersionInfo{
			Name:         "gitlab-ci-multi-runner",
			Version:      "1.3.0~beta.26.gbdcb5e6",
			Revision:     "bdcb5e6",
			Branch:       "feature/stats-server",
			GOVersion:    "go1.6.2",
			BuiltAt:      buildAt,
			OS:           "linux",
			Architecture: "amd64",
		},
	}

	encodedData, _ := json.Marshal(data)
	expectedData := []byte("{\"started_at\":\"2016-07-02T00:01:44+02:00\",\"config_reloaded_at\":\"2016-07-02T00:01:44+02:00\",\"builds_count\":5,\"runners_builds_counts\":{\"abc1234\":3,\"def5678\":2},\"uptime\":0.1234,\"version_info\":{\"name\":\"gitlab-ci-multi-runner\",\"version\":\"1.3.0~beta.26.gbdcb5e6\",\"revision\":\"bdcb5e6\",\"branch\":\"feature/stats-server\",\"go_version\":\"go1.6.2\",\"built_at\":\"2016-07-02T00:01:28+02:00\",\"os\":\"linux\",\"architecture\":\"amd64\"}}")

	equal := true
	for i := range encodedData {
		if encodedData[i] != expectedData[i] {
			equal = false
			break
		}
	}

	if !equal {
		t.Error("JSON encoding invalid.\nexpected:\n\t", string(expectedData), "\ngot:\n\t", string(encodedData))
	}
}

var STARTED_AT = "2016-07-02T00:01:44+02:00"
var CONFIG_RELOADED_AT = "2016-07-02T00:01:44+02:00"

type TestRunCommand struct{}

func (t *TestRunCommand) StatsData() StatsData {
	startedAt, _ := time.Parse(time.RFC3339, STARTED_AT)
	configReloadedAt, _ := time.Parse(time.RFC3339, CONFIG_RELOADED_AT)

	return StatsData{
		StartedAt:        startedAt,
		ConfigReloadedAt: configReloadedAt,
		BuildsCount:      5,
		RunnersBuildsCounts: map[string]int{
			"abc1234": 3,
			"def5678": 2,
		},
	}
}

func TestStatsHandlerPrepare(t *testing.T) {
	handler := StatsHandler{
		command: &TestRunCommand{},
	}
	statsData := handler.data()

	if statsData.Uptime <= 0 {
		t.Error("Uptime should not greather than 0")
	}

	if statsData.VersionInfo != common.AppVersion {
		t.Error("Version info is invalid\nexpected:\n\t", common.AppVersion, "\ngot:\n\t", statsData.VersionInfo)
	}

}

func StartServer() (finished, servingStarted chan bool) {
	finished = make(chan bool, 1)
	servingStarted = make(chan bool, 1)

	server := NewStatsServer("tcp://127.0.0.1:64000", &TestRunCommand{}, finished, servingStarted)
	go server.Start()

	return
}

func TestHTTPServer(t *testing.T) {
	finished, servingStarted := StartServer()

	<-servingStarted
	log.Errorln("test")

	res, err := http.Get("http://127.0.0.1:64000")
	if err != nil {
		t.Error(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Error(err)
	}

	data := &StatsData{}
	err = json.Unmarshal(body, data)
	if err != nil {
		t.Error(err)
	}

	finished <- true
}
