// Code generated by mockery v2.53.4. DO NOT EDIT.

package mockpbresource

import mock "github.com/stretchr/testify/mock"

// isWatchEvent_Event is an autogenerated mock type for the isWatchEvent_Event type
type isWatchEvent_Event struct {
	mock.Mock
}

type isWatchEvent_Event_Expecter struct {
	mock *mock.Mock
}

func (_m *isWatchEvent_Event) EXPECT() *isWatchEvent_Event_Expecter {
	return &isWatchEvent_Event_Expecter{mock: &_m.Mock}
}

// isWatchEvent_Event provides a mock function with no fields
func (_m *isWatchEvent_Event) isWatchEvent_Event() {
	_m.Called()
}

// isWatchEvent_Event_isWatchEvent_Event_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'isWatchEvent_Event'
type isWatchEvent_Event_isWatchEvent_Event_Call struct {
	*mock.Call
}

// isWatchEvent_Event is a helper method to define mock.On call
func (_e *isWatchEvent_Event_Expecter) isWatchEvent_Event() *isWatchEvent_Event_isWatchEvent_Event_Call {
	return &isWatchEvent_Event_isWatchEvent_Event_Call{Call: _e.mock.On("isWatchEvent_Event")}
}

func (_c *isWatchEvent_Event_isWatchEvent_Event_Call) Run(run func()) *isWatchEvent_Event_isWatchEvent_Event_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *isWatchEvent_Event_isWatchEvent_Event_Call) Return() *isWatchEvent_Event_isWatchEvent_Event_Call {
	_c.Call.Return()
	return _c
}

func (_c *isWatchEvent_Event_isWatchEvent_Event_Call) RunAndReturn(run func()) *isWatchEvent_Event_isWatchEvent_Event_Call {
	_c.Run(run)
	return _c
}

// newIsWatchEvent_Event creates a new instance of isWatchEvent_Event. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newIsWatchEvent_Event(t interface {
	mock.TestingT
	Cleanup(func())
}) *isWatchEvent_Event {
	mock := &isWatchEvent_Event{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
