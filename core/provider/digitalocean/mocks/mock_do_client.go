// Code generated by mockery v2.52.1. DO NOT EDIT.

package mocks

import (
	context "context"

	godo "github.com/digitalocean/godo"

	mock "github.com/stretchr/testify/mock"
)

// MockDoClient is an autogenerated mock type for the DoClient type
type MockDoClient struct {
	mock.Mock
}

type MockDoClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockDoClient) EXPECT() *MockDoClient_Expecter {
	return &MockDoClient_Expecter{mock: &_m.Mock}
}

// CreateDroplet provides a mock function with given fields: ctx, req
func (_m *MockDoClient) CreateDroplet(ctx context.Context, req *godo.DropletCreateRequest) (*godo.Droplet, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for CreateDroplet")
	}

	var r0 *godo.Droplet
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *godo.DropletCreateRequest) (*godo.Droplet, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *godo.DropletCreateRequest) *godo.Droplet); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Droplet)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *godo.DropletCreateRequest) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockDoClient_CreateDroplet_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateDroplet'
type MockDoClient_CreateDroplet_Call struct {
	*mock.Call
}

// CreateDroplet is a helper method to define mock.On call
//   - ctx context.Context
//   - req *godo.DropletCreateRequest
func (_e *MockDoClient_Expecter) CreateDroplet(ctx interface{}, req interface{}) *MockDoClient_CreateDroplet_Call {
	return &MockDoClient_CreateDroplet_Call{Call: _e.mock.On("CreateDroplet", ctx, req)}
}

func (_c *MockDoClient_CreateDroplet_Call) Run(run func(ctx context.Context, req *godo.DropletCreateRequest)) *MockDoClient_CreateDroplet_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*godo.DropletCreateRequest))
	})
	return _c
}

func (_c *MockDoClient_CreateDroplet_Call) Return(_a0 *godo.Droplet, _a1 error) *MockDoClient_CreateDroplet_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockDoClient_CreateDroplet_Call) RunAndReturn(run func(context.Context, *godo.DropletCreateRequest) (*godo.Droplet, error)) *MockDoClient_CreateDroplet_Call {
	_c.Call.Return(run)
	return _c
}

// CreateFirewall provides a mock function with given fields: ctx, req
func (_m *MockDoClient) CreateFirewall(ctx context.Context, req *godo.FirewallRequest) (*godo.Firewall, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for CreateFirewall")
	}

	var r0 *godo.Firewall
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *godo.FirewallRequest) (*godo.Firewall, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *godo.FirewallRequest) *godo.Firewall); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Firewall)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *godo.FirewallRequest) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockDoClient_CreateFirewall_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateFirewall'
type MockDoClient_CreateFirewall_Call struct {
	*mock.Call
}

// CreateFirewall is a helper method to define mock.On call
//   - ctx context.Context
//   - req *godo.FirewallRequest
func (_e *MockDoClient_Expecter) CreateFirewall(ctx interface{}, req interface{}) *MockDoClient_CreateFirewall_Call {
	return &MockDoClient_CreateFirewall_Call{Call: _e.mock.On("CreateFirewall", ctx, req)}
}

func (_c *MockDoClient_CreateFirewall_Call) Run(run func(ctx context.Context, req *godo.FirewallRequest)) *MockDoClient_CreateFirewall_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*godo.FirewallRequest))
	})
	return _c
}

func (_c *MockDoClient_CreateFirewall_Call) Return(_a0 *godo.Firewall, _a1 error) *MockDoClient_CreateFirewall_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockDoClient_CreateFirewall_Call) RunAndReturn(run func(context.Context, *godo.FirewallRequest) (*godo.Firewall, error)) *MockDoClient_CreateFirewall_Call {
	_c.Call.Return(run)
	return _c
}

// CreateKey provides a mock function with given fields: ctx, req
func (_m *MockDoClient) CreateKey(ctx context.Context, req *godo.KeyCreateRequest) (*godo.Key, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for CreateKey")
	}

	var r0 *godo.Key
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *godo.KeyCreateRequest) (*godo.Key, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *godo.KeyCreateRequest) *godo.Key); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Key)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *godo.KeyCreateRequest) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockDoClient_CreateKey_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateKey'
type MockDoClient_CreateKey_Call struct {
	*mock.Call
}

// CreateKey is a helper method to define mock.On call
//   - ctx context.Context
//   - req *godo.KeyCreateRequest
func (_e *MockDoClient_Expecter) CreateKey(ctx interface{}, req interface{}) *MockDoClient_CreateKey_Call {
	return &MockDoClient_CreateKey_Call{Call: _e.mock.On("CreateKey", ctx, req)}
}

func (_c *MockDoClient_CreateKey_Call) Run(run func(ctx context.Context, req *godo.KeyCreateRequest)) *MockDoClient_CreateKey_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*godo.KeyCreateRequest))
	})
	return _c
}

func (_c *MockDoClient_CreateKey_Call) Return(_a0 *godo.Key, _a1 error) *MockDoClient_CreateKey_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockDoClient_CreateKey_Call) RunAndReturn(run func(context.Context, *godo.KeyCreateRequest) (*godo.Key, error)) *MockDoClient_CreateKey_Call {
	_c.Call.Return(run)
	return _c
}

// CreateTag provides a mock function with given fields: ctx, req
func (_m *MockDoClient) CreateTag(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for CreateTag")
	}

	var r0 *godo.Tag
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *godo.TagCreateRequest) (*godo.Tag, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *godo.TagCreateRequest) *godo.Tag); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Tag)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *godo.TagCreateRequest) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockDoClient_CreateTag_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateTag'
type MockDoClient_CreateTag_Call struct {
	*mock.Call
}

// CreateTag is a helper method to define mock.On call
//   - ctx context.Context
//   - req *godo.TagCreateRequest
func (_e *MockDoClient_Expecter) CreateTag(ctx interface{}, req interface{}) *MockDoClient_CreateTag_Call {
	return &MockDoClient_CreateTag_Call{Call: _e.mock.On("CreateTag", ctx, req)}
}

func (_c *MockDoClient_CreateTag_Call) Run(run func(ctx context.Context, req *godo.TagCreateRequest)) *MockDoClient_CreateTag_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*godo.TagCreateRequest))
	})
	return _c
}

func (_c *MockDoClient_CreateTag_Call) Return(_a0 *godo.Tag, _a1 error) *MockDoClient_CreateTag_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockDoClient_CreateTag_Call) RunAndReturn(run func(context.Context, *godo.TagCreateRequest) (*godo.Tag, error)) *MockDoClient_CreateTag_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteDropletByID provides a mock function with given fields: ctx, id
func (_m *MockDoClient) DeleteDropletByID(ctx context.Context, id int) error {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for DeleteDropletByID")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int) error); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockDoClient_DeleteDropletByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteDropletByID'
type MockDoClient_DeleteDropletByID_Call struct {
	*mock.Call
}

// DeleteDropletByID is a helper method to define mock.On call
//   - ctx context.Context
//   - id int
func (_e *MockDoClient_Expecter) DeleteDropletByID(ctx interface{}, id interface{}) *MockDoClient_DeleteDropletByID_Call {
	return &MockDoClient_DeleteDropletByID_Call{Call: _e.mock.On("DeleteDropletByID", ctx, id)}
}

func (_c *MockDoClient_DeleteDropletByID_Call) Run(run func(ctx context.Context, id int)) *MockDoClient_DeleteDropletByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int))
	})
	return _c
}

func (_c *MockDoClient_DeleteDropletByID_Call) Return(_a0 error) *MockDoClient_DeleteDropletByID_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockDoClient_DeleteDropletByID_Call) RunAndReturn(run func(context.Context, int) error) *MockDoClient_DeleteDropletByID_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteDropletByTag provides a mock function with given fields: ctx, tag
func (_m *MockDoClient) DeleteDropletByTag(ctx context.Context, tag string) error {
	ret := _m.Called(ctx, tag)

	if len(ret) == 0 {
		panic("no return value specified for DeleteDropletByTag")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, tag)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockDoClient_DeleteDropletByTag_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteDropletByTag'
type MockDoClient_DeleteDropletByTag_Call struct {
	*mock.Call
}

// DeleteDropletByTag is a helper method to define mock.On call
//   - ctx context.Context
//   - tag string
func (_e *MockDoClient_Expecter) DeleteDropletByTag(ctx interface{}, tag interface{}) *MockDoClient_DeleteDropletByTag_Call {
	return &MockDoClient_DeleteDropletByTag_Call{Call: _e.mock.On("DeleteDropletByTag", ctx, tag)}
}

func (_c *MockDoClient_DeleteDropletByTag_Call) Run(run func(ctx context.Context, tag string)) *MockDoClient_DeleteDropletByTag_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockDoClient_DeleteDropletByTag_Call) Return(_a0 error) *MockDoClient_DeleteDropletByTag_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockDoClient_DeleteDropletByTag_Call) RunAndReturn(run func(context.Context, string) error) *MockDoClient_DeleteDropletByTag_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteFirewall provides a mock function with given fields: ctx, firewallID
func (_m *MockDoClient) DeleteFirewall(ctx context.Context, firewallID string) error {
	ret := _m.Called(ctx, firewallID)

	if len(ret) == 0 {
		panic("no return value specified for DeleteFirewall")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, firewallID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockDoClient_DeleteFirewall_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteFirewall'
type MockDoClient_DeleteFirewall_Call struct {
	*mock.Call
}

// DeleteFirewall is a helper method to define mock.On call
//   - ctx context.Context
//   - firewallID string
func (_e *MockDoClient_Expecter) DeleteFirewall(ctx interface{}, firewallID interface{}) *MockDoClient_DeleteFirewall_Call {
	return &MockDoClient_DeleteFirewall_Call{Call: _e.mock.On("DeleteFirewall", ctx, firewallID)}
}

func (_c *MockDoClient_DeleteFirewall_Call) Run(run func(ctx context.Context, firewallID string)) *MockDoClient_DeleteFirewall_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockDoClient_DeleteFirewall_Call) Return(_a0 error) *MockDoClient_DeleteFirewall_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockDoClient_DeleteFirewall_Call) RunAndReturn(run func(context.Context, string) error) *MockDoClient_DeleteFirewall_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteKeyByFingerprint provides a mock function with given fields: ctx, fingerprint
func (_m *MockDoClient) DeleteKeyByFingerprint(ctx context.Context, fingerprint string) error {
	ret := _m.Called(ctx, fingerprint)

	if len(ret) == 0 {
		panic("no return value specified for DeleteKeyByFingerprint")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, fingerprint)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockDoClient_DeleteKeyByFingerprint_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteKeyByFingerprint'
type MockDoClient_DeleteKeyByFingerprint_Call struct {
	*mock.Call
}

// DeleteKeyByFingerprint is a helper method to define mock.On call
//   - ctx context.Context
//   - fingerprint string
func (_e *MockDoClient_Expecter) DeleteKeyByFingerprint(ctx interface{}, fingerprint interface{}) *MockDoClient_DeleteKeyByFingerprint_Call {
	return &MockDoClient_DeleteKeyByFingerprint_Call{Call: _e.mock.On("DeleteKeyByFingerprint", ctx, fingerprint)}
}

func (_c *MockDoClient_DeleteKeyByFingerprint_Call) Run(run func(ctx context.Context, fingerprint string)) *MockDoClient_DeleteKeyByFingerprint_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockDoClient_DeleteKeyByFingerprint_Call) Return(_a0 error) *MockDoClient_DeleteKeyByFingerprint_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockDoClient_DeleteKeyByFingerprint_Call) RunAndReturn(run func(context.Context, string) error) *MockDoClient_DeleteKeyByFingerprint_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteTag provides a mock function with given fields: ctx, tag
func (_m *MockDoClient) DeleteTag(ctx context.Context, tag string) error {
	ret := _m.Called(ctx, tag)

	if len(ret) == 0 {
		panic("no return value specified for DeleteTag")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, tag)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockDoClient_DeleteTag_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteTag'
type MockDoClient_DeleteTag_Call struct {
	*mock.Call
}

// DeleteTag is a helper method to define mock.On call
//   - ctx context.Context
//   - tag string
func (_e *MockDoClient_Expecter) DeleteTag(ctx interface{}, tag interface{}) *MockDoClient_DeleteTag_Call {
	return &MockDoClient_DeleteTag_Call{Call: _e.mock.On("DeleteTag", ctx, tag)}
}

func (_c *MockDoClient_DeleteTag_Call) Run(run func(ctx context.Context, tag string)) *MockDoClient_DeleteTag_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockDoClient_DeleteTag_Call) Return(_a0 error) *MockDoClient_DeleteTag_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockDoClient_DeleteTag_Call) RunAndReturn(run func(context.Context, string) error) *MockDoClient_DeleteTag_Call {
	_c.Call.Return(run)
	return _c
}

// GetDroplet provides a mock function with given fields: ctx, dropletID
func (_m *MockDoClient) GetDroplet(ctx context.Context, dropletID int) (*godo.Droplet, error) {
	ret := _m.Called(ctx, dropletID)

	if len(ret) == 0 {
		panic("no return value specified for GetDroplet")
	}

	var r0 *godo.Droplet
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int) (*godo.Droplet, error)); ok {
		return rf(ctx, dropletID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int) *godo.Droplet); ok {
		r0 = rf(ctx, dropletID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Droplet)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int) error); ok {
		r1 = rf(ctx, dropletID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockDoClient_GetDroplet_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetDroplet'
type MockDoClient_GetDroplet_Call struct {
	*mock.Call
}

// GetDroplet is a helper method to define mock.On call
//   - ctx context.Context
//   - dropletID int
func (_e *MockDoClient_Expecter) GetDroplet(ctx interface{}, dropletID interface{}) *MockDoClient_GetDroplet_Call {
	return &MockDoClient_GetDroplet_Call{Call: _e.mock.On("GetDroplet", ctx, dropletID)}
}

func (_c *MockDoClient_GetDroplet_Call) Run(run func(ctx context.Context, dropletID int)) *MockDoClient_GetDroplet_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int))
	})
	return _c
}

func (_c *MockDoClient_GetDroplet_Call) Return(_a0 *godo.Droplet, _a1 error) *MockDoClient_GetDroplet_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockDoClient_GetDroplet_Call) RunAndReturn(run func(context.Context, int) (*godo.Droplet, error)) *MockDoClient_GetDroplet_Call {
	_c.Call.Return(run)
	return _c
}

// GetFirewall provides a mock function with given fields: ctx, firewallID
func (_m *MockDoClient) GetFirewall(ctx context.Context, firewallID string) (*godo.Firewall, error) {
	ret := _m.Called(ctx, firewallID)

	if len(ret) == 0 {
		panic("no return value specified for GetFirewall")
	}

	var r0 *godo.Firewall
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*godo.Firewall, error)); ok {
		return rf(ctx, firewallID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *godo.Firewall); ok {
		r0 = rf(ctx, firewallID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Firewall)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, firewallID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockDoClient_GetFirewall_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetFirewall'
type MockDoClient_GetFirewall_Call struct {
	*mock.Call
}

// GetFirewall is a helper method to define mock.On call
//   - ctx context.Context
//   - firewallID string
func (_e *MockDoClient_Expecter) GetFirewall(ctx interface{}, firewallID interface{}) *MockDoClient_GetFirewall_Call {
	return &MockDoClient_GetFirewall_Call{Call: _e.mock.On("GetFirewall", ctx, firewallID)}
}

func (_c *MockDoClient_GetFirewall_Call) Run(run func(ctx context.Context, firewallID string)) *MockDoClient_GetFirewall_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockDoClient_GetFirewall_Call) Return(_a0 *godo.Firewall, _a1 error) *MockDoClient_GetFirewall_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockDoClient_GetFirewall_Call) RunAndReturn(run func(context.Context, string) (*godo.Firewall, error)) *MockDoClient_GetFirewall_Call {
	_c.Call.Return(run)
	return _c
}

// GetKeyByFingerprint provides a mock function with given fields: ctx, fingerprint
func (_m *MockDoClient) GetKeyByFingerprint(ctx context.Context, fingerprint string) (*godo.Key, error) {
	ret := _m.Called(ctx, fingerprint)

	if len(ret) == 0 {
		panic("no return value specified for GetKeyByFingerprint")
	}

	var r0 *godo.Key
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*godo.Key, error)); ok {
		return rf(ctx, fingerprint)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *godo.Key); ok {
		r0 = rf(ctx, fingerprint)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Key)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, fingerprint)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockDoClient_GetKeyByFingerprint_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetKeyByFingerprint'
type MockDoClient_GetKeyByFingerprint_Call struct {
	*mock.Call
}

// GetKeyByFingerprint is a helper method to define mock.On call
//   - ctx context.Context
//   - fingerprint string
func (_e *MockDoClient_Expecter) GetKeyByFingerprint(ctx interface{}, fingerprint interface{}) *MockDoClient_GetKeyByFingerprint_Call {
	return &MockDoClient_GetKeyByFingerprint_Call{Call: _e.mock.On("GetKeyByFingerprint", ctx, fingerprint)}
}

func (_c *MockDoClient_GetKeyByFingerprint_Call) Run(run func(ctx context.Context, fingerprint string)) *MockDoClient_GetKeyByFingerprint_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockDoClient_GetKeyByFingerprint_Call) Return(_a0 *godo.Key, _a1 error) *MockDoClient_GetKeyByFingerprint_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockDoClient_GetKeyByFingerprint_Call) RunAndReturn(run func(context.Context, string) (*godo.Key, error)) *MockDoClient_GetKeyByFingerprint_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockDoClient creates a new instance of MockDoClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockDoClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockDoClient {
	mock := &MockDoClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
