package service_helpers

import (
	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
)

func New(i service.Interface, c *service.Config) (service.Service, error) {
	s, err := service.New(i, c)
	if err == service.ErrNoServiceSystemDetected {
		logrus.Warningln("No service system detected. Some features may not work!")

		return &SimpleService{
			i: i,
			c: c,
		}, nil
	}
	return s, err
}
