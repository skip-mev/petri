// Code generated by mockery v2.47.0. DO NOT EDIT.

package mocks

import (
	context "context"

	godo "github.com/digitalocean/godo"

	mock "github.com/stretchr/testify/mock"
)

// DoClient is an autogenerated mock type for the DoClient type
type DoClient struct {
	mock.Mock
}

// CreateDroplet provides a mock function with given fields: ctx, req
func (_m *DoClient) CreateDroplet(ctx context.Context, req *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for CreateDroplet")
	}

	var r0 *godo.Droplet
	var r1 *godo.Response
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *godo.DropletCreateRequest) *godo.Droplet); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Droplet)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *godo.DropletCreateRequest) *godo.Response); ok {
		r1 = rf(ctx, req)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*godo.Response)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, *godo.DropletCreateRequest) error); ok {
		r2 = rf(ctx, req)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// CreateFirewall provides a mock function with given fields: ctx, req
func (_m *DoClient) CreateFirewall(ctx context.Context, req *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for CreateFirewall")
	}

	var r0 *godo.Firewall
	var r1 *godo.Response
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *godo.FirewallRequest) *godo.Firewall); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Firewall)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *godo.FirewallRequest) *godo.Response); ok {
		r1 = rf(ctx, req)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*godo.Response)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, *godo.FirewallRequest) error); ok {
		r2 = rf(ctx, req)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// CreateKey provides a mock function with given fields: ctx, req
func (_m *DoClient) CreateKey(ctx context.Context, req *godo.KeyCreateRequest) (*godo.Key, *godo.Response, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for CreateKey")
	}

	var r0 *godo.Key
	var r1 *godo.Response
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *godo.KeyCreateRequest) (*godo.Key, *godo.Response, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *godo.KeyCreateRequest) *godo.Key); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Key)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *godo.KeyCreateRequest) *godo.Response); ok {
		r1 = rf(ctx, req)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*godo.Response)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, *godo.KeyCreateRequest) error); ok {
		r2 = rf(ctx, req)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// CreateTag provides a mock function with given fields: ctx, req
func (_m *DoClient) CreateTag(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for CreateTag")
	}

	var r0 *godo.Tag
	var r1 *godo.Response
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *godo.TagCreateRequest) *godo.Tag); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Tag)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *godo.TagCreateRequest) *godo.Response); ok {
		r1 = rf(ctx, req)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*godo.Response)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, *godo.TagCreateRequest) error); ok {
		r2 = rf(ctx, req)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// DeleteDropletByID provides a mock function with given fields: ctx, id
func (_m *DoClient) DeleteDropletByID(ctx context.Context, id int) (*godo.Response, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for DeleteDropletByID")
	}

	var r0 *godo.Response
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int) (*godo.Response, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int) *godo.Response); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Response)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteDropletByTag provides a mock function with given fields: ctx, tag
func (_m *DoClient) DeleteDropletByTag(ctx context.Context, tag string) (*godo.Response, error) {
	ret := _m.Called(ctx, tag)

	if len(ret) == 0 {
		panic("no return value specified for DeleteDropletByTag")
	}

	var r0 *godo.Response
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*godo.Response, error)); ok {
		return rf(ctx, tag)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *godo.Response); ok {
		r0 = rf(ctx, tag)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Response)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, tag)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteFirewall provides a mock function with given fields: ctx, firewallID
func (_m *DoClient) DeleteFirewall(ctx context.Context, firewallID string) (*godo.Response, error) {
	ret := _m.Called(ctx, firewallID)

	if len(ret) == 0 {
		panic("no return value specified for DeleteFirewall")
	}

	var r0 *godo.Response
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*godo.Response, error)); ok {
		return rf(ctx, firewallID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *godo.Response); ok {
		r0 = rf(ctx, firewallID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Response)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, firewallID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteKeyByFingerprint provides a mock function with given fields: ctx, fingerprint
func (_m *DoClient) DeleteKeyByFingerprint(ctx context.Context, fingerprint string) (*godo.Response, error) {
	ret := _m.Called(ctx, fingerprint)

	if len(ret) == 0 {
		panic("no return value specified for DeleteKeyByFingerprint")
	}

	var r0 *godo.Response
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*godo.Response, error)); ok {
		return rf(ctx, fingerprint)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *godo.Response); ok {
		r0 = rf(ctx, fingerprint)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Response)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, fingerprint)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteTag provides a mock function with given fields: ctx, tag
func (_m *DoClient) DeleteTag(ctx context.Context, tag string) (*godo.Response, error) {
	ret := _m.Called(ctx, tag)

	if len(ret) == 0 {
		panic("no return value specified for DeleteTag")
	}

	var r0 *godo.Response
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*godo.Response, error)); ok {
		return rf(ctx, tag)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *godo.Response); ok {
		r0 = rf(ctx, tag)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Response)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, tag)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDroplet provides a mock function with given fields: ctx, dropletID
func (_m *DoClient) GetDroplet(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
	ret := _m.Called(ctx, dropletID)

	if len(ret) == 0 {
		panic("no return value specified for GetDroplet")
	}

	var r0 *godo.Droplet
	var r1 *godo.Response
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, int) (*godo.Droplet, *godo.Response, error)); ok {
		return rf(ctx, dropletID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int) *godo.Droplet); ok {
		r0 = rf(ctx, dropletID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Droplet)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int) *godo.Response); ok {
		r1 = rf(ctx, dropletID)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*godo.Response)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, int) error); ok {
		r2 = rf(ctx, dropletID)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetKeyByFingerprint provides a mock function with given fields: ctx, fingerprint
func (_m *DoClient) GetKeyByFingerprint(ctx context.Context, fingerprint string) (*godo.Key, *godo.Response, error) {
	ret := _m.Called(ctx, fingerprint)

	if len(ret) == 0 {
		panic("no return value specified for GetKeyByFingerprint")
	}

	var r0 *godo.Key
	var r1 *godo.Response
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*godo.Key, *godo.Response, error)); ok {
		return rf(ctx, fingerprint)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *godo.Key); ok {
		r0 = rf(ctx, fingerprint)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*godo.Key)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) *godo.Response); ok {
		r1 = rf(ctx, fingerprint)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*godo.Response)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, string) error); ok {
		r2 = rf(ctx, fingerprint)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// NewDoClient creates a new instance of DoClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewDoClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *DoClient {
	mock := &DoClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}