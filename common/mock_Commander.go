package common

import "github.com/stretchr/testify/mock"

import "github.com/urfave/cli"

type MockCommander struct {
	mock.Mock
}

func (m *MockCommander) Execute(c *cli.Context) {
	m.Called(c)
}
