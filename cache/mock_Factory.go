// Code generated by mockery v2.53.3. DO NOT EDIT.

package cache

import (
	mock "github.com/stretchr/testify/mock"
	common "gitlab.com/gitlab-org/gitlab-runner/common"

	time "time"
)

// MockFactory is an autogenerated mock type for the Factory type
type MockFactory struct {
	mock.Mock
}

type MockFactory_Expecter struct {
	mock *mock.Mock
}

func (_m *MockFactory) EXPECT() *MockFactory_Expecter {
	return &MockFactory_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: config, timeout, objectName
func (_m *MockFactory) Execute(config *common.CacheConfig, timeout time.Duration, objectName string) (Adapter, error) {
	ret := _m.Called(config, timeout, objectName)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 Adapter
	var r1 error
	if rf, ok := ret.Get(0).(func(*common.CacheConfig, time.Duration, string) (Adapter, error)); ok {
		return rf(config, timeout, objectName)
	}
	if rf, ok := ret.Get(0).(func(*common.CacheConfig, time.Duration, string) Adapter); ok {
		r0 = rf(config, timeout, objectName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(Adapter)
		}
	}

	if rf, ok := ret.Get(1).(func(*common.CacheConfig, time.Duration, string) error); ok {
		r1 = rf(config, timeout, objectName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockFactory_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type MockFactory_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - config *common.CacheConfig
//   - timeout time.Duration
//   - objectName string
func (_e *MockFactory_Expecter) Execute(config interface{}, timeout interface{}, objectName interface{}) *MockFactory_Execute_Call {
	return &MockFactory_Execute_Call{Call: _e.mock.On("Execute", config, timeout, objectName)}
}

func (_c *MockFactory_Execute_Call) Run(run func(config *common.CacheConfig, timeout time.Duration, objectName string)) *MockFactory_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*common.CacheConfig), args[1].(time.Duration), args[2].(string))
	})
	return _c
}

func (_c *MockFactory_Execute_Call) Return(_a0 Adapter, _a1 error) *MockFactory_Execute_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockFactory_Execute_Call) RunAndReturn(run func(*common.CacheConfig, time.Duration, string) (Adapter, error)) *MockFactory_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockFactory creates a new instance of MockFactory. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockFactory {
	mock := &MockFactory{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
