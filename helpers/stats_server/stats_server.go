package stats_server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
)

type StatsData struct {
	StartedAt        time.Time `json:"started_at"`
	ConfigReloadedAt time.Time `json:"config_reloaded_at"`
	BuildsCount      int       `json:"builds_count"`

	RunnersBuildsCounts map[string]int `json:"runners_builds_counts"`

	Uptime      float64               `json:"uptime"`
	VersionInfo common.AppVersionInfo `json:"version_info"`
}

func (data *StatsData) Prepare() {
	data.StartedAt = data.StartedAt.Round(time.Second)
	data.ConfigReloadedAt = data.ConfigReloadedAt.Round(time.Second)
	data.Uptime, _ = strconv.ParseFloat(fmt.Sprintf("%.4f", time.Since(data.StartedAt).Hours()), 64)
	data.VersionInfo = common.AppVersion

	runnersBuildsCounts := map[string]int{}
	for token, count := range data.RunnersBuildsCounts {
		runnersBuildsCounts[helpers.ShortenToken(token)] = count
	}
	data.RunnersBuildsCounts = runnersBuildsCounts
}

type RunCommand interface {
	StatsData() StatsData
}

type StatsHandler struct {
	command RunCommand
}

func (h *StatsHandler) data() StatsData {
	data := h.command.StatsData()
	data.Prepare()
	return data
}

func (h *StatsHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	bytes, err := json.Marshal(h.data())
	if err != nil {
		log.WithError(err).Errorln("Error with StatsData marshalling to JSON")
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(bytes)
}

type StatsServer struct {
	address        string
	command        RunCommand
	runFinished    chan bool
	servingStarted chan bool
}

type StatsServerNotEnabledError struct {
	Inner error
}

func (e *StatsServerNotEnabledError) Error() string {
	if e.Inner == nil {
		return "StatsServer not enabled"
	}
	return e.Inner.Error()
}

func (server *StatsServer) Start() {
	socketNet, socketAddress, err := server.parseAddress()
	if err != nil {
		if _, ok := err.(*StatsServerNotEnabledError); ok {
			log.Infoln("StatsServer disabled")
		} else {
			log.WithError(err).Warningln("Can't start StatsServer")
		}
		return
	}

	listener, err := net.Listen(socketNet, socketAddress)
	if err != nil {
		log.WithError(err).Errorln("StatsServer listner failure")
		return
	}

	if socketNet == "unix" {
		defer os.Remove(socketAddress)
	}

	srv := &http.Server{
		Handler: &StatsHandler{
			command: server.command,
		},
	}

	log.WithField("socket", listener.Addr()).Infoln("Starting StatsServer...")
	go srv.Serve(listener)

	server.servingStarted <- true

	<-server.runFinished
	log.Infoln("Stopping StatsServer...")
	server.runFinished <- true
}

func (server *StatsServer) parseAddress() (net, address string, err error) {
	if server.address == "" {
		err = &StatsServerNotEnabledError{}
		return
	}

	parts := strings.SplitN(server.address, "://", 2)
	if len(parts) < 2 {
		err = errors.New("Invalid StatsServer socket address format")
		return
	}

	switch parts[0] {
	case "tcp", "unix":
		net = parts[0]
	default:
		err = fmt.Errorf("Invalid network type: %s", parts[0])
		return
	}

	net = parts[0]
	address = parts[1]

	return
}

func NewStatsServer(address string, runCommand RunCommand, runFinished chan bool, servingStarted chan bool) *StatsServer {
	return &StatsServer{
		address:        address,
		command:        runCommand,
		runFinished:    runFinished,
		servingStarted: servingStarted,
	}
}
