// Code generated by mockery. DO NOT EDIT.

package mocks

import (
	context "context"
	models "planeo/services/core/internal/resources/request/models"

	mock "github.com/stretchr/testify/mock"
)

// MockRequestRepositoryInterface is an autogenerated mock type for the RequestRepositoryInterface type
type MockRequestRepositoryInterface struct {
	mock.Mock
}

type MockRequestRepositoryInterface_Expecter struct {
	mock *mock.Mock
}

func (_m *MockRequestRepositoryInterface) EXPECT() *MockRequestRepositoryInterface_Expecter {
	return &MockRequestRepositoryInterface_Expecter{mock: &_m.Mock}
}

// CreateRequest provides a mock function with given fields: ctx, _a1
func (_m *MockRequestRepositoryInterface) CreateRequest(ctx context.Context, _a1 models.NewRequest) (int, error) {
	ret := _m.Called(ctx, _a1)

	if len(ret) == 0 {
		panic("no return value specified for CreateRequest")
	}

	var r0 int
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, models.NewRequest) (int, error)); ok {
		return rf(ctx, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, models.NewRequest) int); ok {
		r0 = rf(ctx, _a1)
	} else {
		r0 = ret.Get(0).(int)
	}

	if rf, ok := ret.Get(1).(func(context.Context, models.NewRequest) error); ok {
		r1 = rf(ctx, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRequestRepositoryInterface_CreateRequest_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateRequest'
type MockRequestRepositoryInterface_CreateRequest_Call struct {
	*mock.Call
}

// CreateRequest is a helper method to define mock.On call
//   - ctx context.Context
//   - _a1 models.NewRequest
func (_e *MockRequestRepositoryInterface_Expecter) CreateRequest(ctx interface{}, _a1 interface{}) *MockRequestRepositoryInterface_CreateRequest_Call {
	return &MockRequestRepositoryInterface_CreateRequest_Call{Call: _e.mock.On("CreateRequest", ctx, _a1)}
}

func (_c *MockRequestRepositoryInterface_CreateRequest_Call) Run(run func(ctx context.Context, _a1 models.NewRequest)) *MockRequestRepositoryInterface_CreateRequest_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(models.NewRequest))
	})
	return _c
}

func (_c *MockRequestRepositoryInterface_CreateRequest_Call) Return(_a0 int, _a1 error) *MockRequestRepositoryInterface_CreateRequest_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRequestRepositoryInterface_CreateRequest_Call) RunAndReturn(run func(context.Context, models.NewRequest) (int, error)) *MockRequestRepositoryInterface_CreateRequest_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteRequest provides a mock function with given fields: ctx, organizationId, requestId
func (_m *MockRequestRepositoryInterface) DeleteRequest(ctx context.Context, organizationId int, requestId int) error {
	ret := _m.Called(ctx, organizationId, requestId)

	if len(ret) == 0 {
		panic("no return value specified for DeleteRequest")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int, int) error); ok {
		r0 = rf(ctx, organizationId, requestId)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockRequestRepositoryInterface_DeleteRequest_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteRequest'
type MockRequestRepositoryInterface_DeleteRequest_Call struct {
	*mock.Call
}

// DeleteRequest is a helper method to define mock.On call
//   - ctx context.Context
//   - organizationId int
//   - requestId int
func (_e *MockRequestRepositoryInterface_Expecter) DeleteRequest(ctx interface{}, organizationId interface{}, requestId interface{}) *MockRequestRepositoryInterface_DeleteRequest_Call {
	return &MockRequestRepositoryInterface_DeleteRequest_Call{Call: _e.mock.On("DeleteRequest", ctx, organizationId, requestId)}
}

func (_c *MockRequestRepositoryInterface_DeleteRequest_Call) Run(run func(ctx context.Context, organizationId int, requestId int)) *MockRequestRepositoryInterface_DeleteRequest_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int), args[2].(int))
	})
	return _c
}

func (_c *MockRequestRepositoryInterface_DeleteRequest_Call) Return(_a0 error) *MockRequestRepositoryInterface_DeleteRequest_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockRequestRepositoryInterface_DeleteRequest_Call) RunAndReturn(run func(context.Context, int, int) error) *MockRequestRepositoryInterface_DeleteRequest_Call {
	_c.Call.Return(run)
	return _c
}

// GetRequest provides a mock function with given fields: ctx, organizationId, requestId
func (_m *MockRequestRepositoryInterface) GetRequest(ctx context.Context, organizationId int, requestId int) (models.Request, error) {
	ret := _m.Called(ctx, organizationId, requestId)

	if len(ret) == 0 {
		panic("no return value specified for GetRequest")
	}

	var r0 models.Request
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int, int) (models.Request, error)); ok {
		return rf(ctx, organizationId, requestId)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int, int) models.Request); ok {
		r0 = rf(ctx, organizationId, requestId)
	} else {
		r0 = ret.Get(0).(models.Request)
	}

	if rf, ok := ret.Get(1).(func(context.Context, int, int) error); ok {
		r1 = rf(ctx, organizationId, requestId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRequestRepositoryInterface_GetRequest_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRequest'
type MockRequestRepositoryInterface_GetRequest_Call struct {
	*mock.Call
}

// GetRequest is a helper method to define mock.On call
//   - ctx context.Context
//   - organizationId int
//   - requestId int
func (_e *MockRequestRepositoryInterface_Expecter) GetRequest(ctx interface{}, organizationId interface{}, requestId interface{}) *MockRequestRepositoryInterface_GetRequest_Call {
	return &MockRequestRepositoryInterface_GetRequest_Call{Call: _e.mock.On("GetRequest", ctx, organizationId, requestId)}
}

func (_c *MockRequestRepositoryInterface_GetRequest_Call) Run(run func(ctx context.Context, organizationId int, requestId int)) *MockRequestRepositoryInterface_GetRequest_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int), args[2].(int))
	})
	return _c
}

func (_c *MockRequestRepositoryInterface_GetRequest_Call) Return(_a0 models.Request, _a1 error) *MockRequestRepositoryInterface_GetRequest_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRequestRepositoryInterface_GetRequest_Call) RunAndReturn(run func(context.Context, int, int) (models.Request, error)) *MockRequestRepositoryInterface_GetRequest_Call {
	_c.Call.Return(run)
	return _c
}

// GetRequests provides a mock function with given fields: ctx, organizationId, cursor, limit, getClosed
func (_m *MockRequestRepositoryInterface) GetRequests(ctx context.Context, organizationId int, cursor int, limit int, getClosed bool) ([]models.Request, error) {
	ret := _m.Called(ctx, organizationId, cursor, limit, getClosed)

	if len(ret) == 0 {
		panic("no return value specified for GetRequests")
	}

	var r0 []models.Request
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int, int, int, bool) ([]models.Request, error)); ok {
		return rf(ctx, organizationId, cursor, limit, getClosed)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int, int, int, bool) []models.Request); ok {
		r0 = rf(ctx, organizationId, cursor, limit, getClosed)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]models.Request)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int, int, int, bool) error); ok {
		r1 = rf(ctx, organizationId, cursor, limit, getClosed)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRequestRepositoryInterface_GetRequests_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRequests'
type MockRequestRepositoryInterface_GetRequests_Call struct {
	*mock.Call
}

// GetRequests is a helper method to define mock.On call
//   - ctx context.Context
//   - organizationId int
//   - cursor int
//   - limit int
//   - getClosed bool
func (_e *MockRequestRepositoryInterface_Expecter) GetRequests(ctx interface{}, organizationId interface{}, cursor interface{}, limit interface{}, getClosed interface{}) *MockRequestRepositoryInterface_GetRequests_Call {
	return &MockRequestRepositoryInterface_GetRequests_Call{Call: _e.mock.On("GetRequests", ctx, organizationId, cursor, limit, getClosed)}
}

func (_c *MockRequestRepositoryInterface_GetRequests_Call) Run(run func(ctx context.Context, organizationId int, cursor int, limit int, getClosed bool)) *MockRequestRepositoryInterface_GetRequests_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int), args[2].(int), args[3].(int), args[4].(bool))
	})
	return _c
}

func (_c *MockRequestRepositoryInterface_GetRequests_Call) Return(_a0 []models.Request, _a1 error) *MockRequestRepositoryInterface_GetRequests_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRequestRepositoryInterface_GetRequests_Call) RunAndReturn(run func(context.Context, int, int, int, bool) ([]models.Request, error)) *MockRequestRepositoryInterface_GetRequests_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateRequest provides a mock function with given fields: ctx, _a1
func (_m *MockRequestRepositoryInterface) UpdateRequest(ctx context.Context, _a1 models.UpdateRequest) error {
	ret := _m.Called(ctx, _a1)

	if len(ret) == 0 {
		panic("no return value specified for UpdateRequest")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, models.UpdateRequest) error); ok {
		r0 = rf(ctx, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockRequestRepositoryInterface_UpdateRequest_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateRequest'
type MockRequestRepositoryInterface_UpdateRequest_Call struct {
	*mock.Call
}

// UpdateRequest is a helper method to define mock.On call
//   - ctx context.Context
//   - _a1 models.UpdateRequest
func (_e *MockRequestRepositoryInterface_Expecter) UpdateRequest(ctx interface{}, _a1 interface{}) *MockRequestRepositoryInterface_UpdateRequest_Call {
	return &MockRequestRepositoryInterface_UpdateRequest_Call{Call: _e.mock.On("UpdateRequest", ctx, _a1)}
}

func (_c *MockRequestRepositoryInterface_UpdateRequest_Call) Run(run func(ctx context.Context, _a1 models.UpdateRequest)) *MockRequestRepositoryInterface_UpdateRequest_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(models.UpdateRequest))
	})
	return _c
}

func (_c *MockRequestRepositoryInterface_UpdateRequest_Call) Return(_a0 error) *MockRequestRepositoryInterface_UpdateRequest_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockRequestRepositoryInterface_UpdateRequest_Call) RunAndReturn(run func(context.Context, models.UpdateRequest) error) *MockRequestRepositoryInterface_UpdateRequest_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockRequestRepositoryInterface creates a new instance of MockRequestRepositoryInterface. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockRequestRepositoryInterface(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockRequestRepositoryInterface {
	mock := &MockRequestRepositoryInterface{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
