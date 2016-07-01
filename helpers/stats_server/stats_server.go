package stats_server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

type StatsData struct {
	StartedAt        time.Time `json:"started_at"`
	ConfigReloadedAt time.Time `json:"config_reloaded_at"`
	BuildsCount      int       `json:"builds_count"`
}

type RunCommand interface {
	StatsData() StatsData
}

type StatsHandler struct {
	command RunCommand
}

func (h *StatsHandler) data() StatsData {
	return h.command.StatsData()
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
	address     string
	command     RunCommand
	runFinished chan bool
}

func (server *StatsServer) Start() {
	socketNet, socketAddress, err := server.parseAddress()
	if err != nil {
		log.WithError(err).Warningln("Can't start StatsServer")
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

	<-server.runFinished
	log.Infoln("Stopping StatsServer...")
	server.runFinished <- true
}

func (server *StatsServer) parseAddress() (net, address string, err error) {
	if server.address == "" {
		err = errors.New("ParseSocketPath not set")
		return
	}

	parts := strings.SplitN(server.address, "://", 2)
	if len(parts) < 2 {
		err = errors.New("Invalid ParseSocketPath format")
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

func NewStatsServer(address string, runCommand RunCommand, runFinished chan bool) *StatsServer {
	return &StatsServer{
		address:     address,
		command:     runCommand,
		runFinished: runFinished,
	}
}
