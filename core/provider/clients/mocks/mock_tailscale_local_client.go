// Code generated by mockery v2.52.1. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	ipnstate "tailscale.com/ipn/ipnstate"
)

// MockTailscaleLocalClient is an autogenerated mock type for the TailscaleLocalClient type
type MockTailscaleLocalClient struct {
	mock.Mock
}

type MockTailscaleLocalClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockTailscaleLocalClient) EXPECT() *MockTailscaleLocalClient_Expecter {
	return &MockTailscaleLocalClient_Expecter{mock: &_m.Mock}
}

// Status provides a mock function with given fields: ctx
func (_m *MockTailscaleLocalClient) Status(ctx context.Context) (*ipnstate.Status, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Status")
	}

	var r0 *ipnstate.Status
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*ipnstate.Status, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *ipnstate.Status); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ipnstate.Status)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTailscaleLocalClient_Status_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Status'
type MockTailscaleLocalClient_Status_Call struct {
	*mock.Call
}

// Status is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockTailscaleLocalClient_Expecter) Status(ctx interface{}) *MockTailscaleLocalClient_Status_Call {
	return &MockTailscaleLocalClient_Status_Call{Call: _e.mock.On("Status", ctx)}
}

func (_c *MockTailscaleLocalClient_Status_Call) Run(run func(ctx context.Context)) *MockTailscaleLocalClient_Status_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockTailscaleLocalClient_Status_Call) Return(_a0 *ipnstate.Status, _a1 error) *MockTailscaleLocalClient_Status_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTailscaleLocalClient_Status_Call) RunAndReturn(run func(context.Context) (*ipnstate.Status, error)) *MockTailscaleLocalClient_Status_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockTailscaleLocalClient creates a new instance of MockTailscaleLocalClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockTailscaleLocalClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockTailscaleLocalClient {
	mock := &MockTailscaleLocalClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
