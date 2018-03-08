package service_helpers

import (
	"github.com/ayufan/golang-kardianos-service"
	log "github.com/sirupsen/logrus"
)

func New(i service.Interface, c *service.Config) (service.Service, error) {
	s, err := service.New(i, c)
	if err == service.ErrNoServiceSystemDetected {
		log.Warningln("No service system detected. Some features may not work!")

		return &SimpleService{
			i: i,
			c: c,
		}, nil
	}
	return s, err
}
