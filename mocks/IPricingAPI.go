// Code generated by mockery v2.46.2. DO NOT EDIT.

package mocks

import (
	context "context"

	pricing "github.com/aws/aws-sdk-go-v2/service/pricing"
	mock "github.com/stretchr/testify/mock"
)

// IPricingAPI is an autogenerated mock type for the IPricingAPI type
type IPricingAPI struct {
	mock.Mock
}

// GetProducts provides a mock function with given fields: ctx, params, optFns
func (_m *IPricingAPI) GetProducts(ctx context.Context, params *pricing.GetProductsInput, optFns ...func(*pricing.Options)) (*pricing.GetProductsOutput, error) {
	_va := make([]interface{}, len(optFns))
	for _i := range optFns {
		_va[_i] = optFns[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, params)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for GetProducts")
	}

	var r0 *pricing.GetProductsOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pricing.GetProductsInput, ...func(*pricing.Options)) (*pricing.GetProductsOutput, error)); ok {
		return rf(ctx, params, optFns...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pricing.GetProductsInput, ...func(*pricing.Options)) *pricing.GetProductsOutput); ok {
		r0 = rf(ctx, params, optFns...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pricing.GetProductsOutput)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pricing.GetProductsInput, ...func(*pricing.Options)) error); ok {
		r1 = rf(ctx, params, optFns...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewIPricingAPI creates a new instance of IPricingAPI. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewIPricingAPI(t interface {
	mock.TestingT
	Cleanup(func())
}) *IPricingAPI {
	mock := &IPricingAPI{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
