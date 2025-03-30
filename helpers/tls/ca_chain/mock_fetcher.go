// Code generated by mockery v2.53.3. DO NOT EDIT.

package ca_chain

import mock "github.com/stretchr/testify/mock"

// mockFetcher is an autogenerated mock type for the fetcher type
type mockFetcher struct {
	mock.Mock
}

type mockFetcher_Expecter struct {
	mock *mock.Mock
}

func (_m *mockFetcher) EXPECT() *mockFetcher_Expecter {
	return &mockFetcher_Expecter{mock: &_m.Mock}
}

// Fetch provides a mock function with given fields: url
func (_m *mockFetcher) Fetch(url string) ([]byte, error) {
	ret := _m.Called(url)

	if len(ret) == 0 {
		panic("no return value specified for Fetch")
	}

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func(string) ([]byte, error)); ok {
		return rf(url)
	}
	if rf, ok := ret.Get(0).(func(string) []byte); ok {
		r0 = rf(url)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(url)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockFetcher_Fetch_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Fetch'
type mockFetcher_Fetch_Call struct {
	*mock.Call
}

// Fetch is a helper method to define mock.On call
//   - url string
func (_e *mockFetcher_Expecter) Fetch(url interface{}) *mockFetcher_Fetch_Call {
	return &mockFetcher_Fetch_Call{Call: _e.mock.On("Fetch", url)}
}

func (_c *mockFetcher_Fetch_Call) Run(run func(url string)) *mockFetcher_Fetch_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *mockFetcher_Fetch_Call) Return(_a0 []byte, _a1 error) *mockFetcher_Fetch_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockFetcher_Fetch_Call) RunAndReturn(run func(string) ([]byte, error)) *mockFetcher_Fetch_Call {
	_c.Call.Return(run)
	return _c
}

// newMockFetcher creates a new instance of mockFetcher. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockFetcher(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockFetcher {
	mock := &mockFetcher{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
