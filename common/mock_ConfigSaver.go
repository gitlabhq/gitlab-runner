// Code generated by mockery v2.53.3. DO NOT EDIT.

package common

import mock "github.com/stretchr/testify/mock"

// MockConfigSaver is an autogenerated mock type for the ConfigSaver type
type MockConfigSaver struct {
	mock.Mock
}

type MockConfigSaver_Expecter struct {
	mock *mock.Mock
}

func (_m *MockConfigSaver) EXPECT() *MockConfigSaver_Expecter {
	return &MockConfigSaver_Expecter{mock: &_m.Mock}
}

// Save provides a mock function with given fields: filePath, data
func (_m *MockConfigSaver) Save(filePath string, data []byte) error {
	ret := _m.Called(filePath, data)

	if len(ret) == 0 {
		panic("no return value specified for Save")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, []byte) error); ok {
		r0 = rf(filePath, data)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockConfigSaver_Save_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Save'
type MockConfigSaver_Save_Call struct {
	*mock.Call
}

// Save is a helper method to define mock.On call
//   - filePath string
//   - data []byte
func (_e *MockConfigSaver_Expecter) Save(filePath interface{}, data interface{}) *MockConfigSaver_Save_Call {
	return &MockConfigSaver_Save_Call{Call: _e.mock.On("Save", filePath, data)}
}

func (_c *MockConfigSaver_Save_Call) Run(run func(filePath string, data []byte)) *MockConfigSaver_Save_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].([]byte))
	})
	return _c
}

func (_c *MockConfigSaver_Save_Call) Return(_a0 error) *MockConfigSaver_Save_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockConfigSaver_Save_Call) RunAndReturn(run func(string, []byte) error) *MockConfigSaver_Save_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockConfigSaver creates a new instance of MockConfigSaver. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockConfigSaver(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockConfigSaver {
	mock := &MockConfigSaver{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
