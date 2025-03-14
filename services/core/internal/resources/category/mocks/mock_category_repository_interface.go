// Code generated by mockery. DO NOT EDIT.

package mocks

import (
	context "context"
	models "planeo/services/core/internal/resources/category/models"

	mock "github.com/stretchr/testify/mock"
)

// MockCategoryRepositoryInterface is an autogenerated mock type for the CategoryRepositoryInterface type
type MockCategoryRepositoryInterface struct {
	mock.Mock
}

type MockCategoryRepositoryInterface_Expecter struct {
	mock *mock.Mock
}

func (_m *MockCategoryRepositoryInterface) EXPECT() *MockCategoryRepositoryInterface_Expecter {
	return &MockCategoryRepositoryInterface_Expecter{mock: &_m.Mock}
}

// CreateCategory provides a mock function with given fields: ctx, organizationId, _a2
func (_m *MockCategoryRepositoryInterface) CreateCategory(ctx context.Context, organizationId int, _a2 models.NewCategory) error {
	ret := _m.Called(ctx, organizationId, _a2)

	if len(ret) == 0 {
		panic("no return value specified for CreateCategory")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int, models.NewCategory) error); ok {
		r0 = rf(ctx, organizationId, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockCategoryRepositoryInterface_CreateCategory_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateCategory'
type MockCategoryRepositoryInterface_CreateCategory_Call struct {
	*mock.Call
}

// CreateCategory is a helper method to define mock.On call
//   - ctx context.Context
//   - organizationId int
//   - _a2 models.NewCategory
func (_e *MockCategoryRepositoryInterface_Expecter) CreateCategory(ctx interface{}, organizationId interface{}, _a2 interface{}) *MockCategoryRepositoryInterface_CreateCategory_Call {
	return &MockCategoryRepositoryInterface_CreateCategory_Call{Call: _e.mock.On("CreateCategory", ctx, organizationId, _a2)}
}

func (_c *MockCategoryRepositoryInterface_CreateCategory_Call) Run(run func(ctx context.Context, organizationId int, _a2 models.NewCategory)) *MockCategoryRepositoryInterface_CreateCategory_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int), args[2].(models.NewCategory))
	})
	return _c
}

func (_c *MockCategoryRepositoryInterface_CreateCategory_Call) Return(_a0 error) *MockCategoryRepositoryInterface_CreateCategory_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockCategoryRepositoryInterface_CreateCategory_Call) RunAndReturn(run func(context.Context, int, models.NewCategory) error) *MockCategoryRepositoryInterface_CreateCategory_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteCategory provides a mock function with given fields: ctx, organizationId, categoryId
func (_m *MockCategoryRepositoryInterface) DeleteCategory(ctx context.Context, organizationId int, categoryId int) error {
	ret := _m.Called(ctx, organizationId, categoryId)

	if len(ret) == 0 {
		panic("no return value specified for DeleteCategory")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int, int) error); ok {
		r0 = rf(ctx, organizationId, categoryId)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockCategoryRepositoryInterface_DeleteCategory_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteCategory'
type MockCategoryRepositoryInterface_DeleteCategory_Call struct {
	*mock.Call
}

// DeleteCategory is a helper method to define mock.On call
//   - ctx context.Context
//   - organizationId int
//   - categoryId int
func (_e *MockCategoryRepositoryInterface_Expecter) DeleteCategory(ctx interface{}, organizationId interface{}, categoryId interface{}) *MockCategoryRepositoryInterface_DeleteCategory_Call {
	return &MockCategoryRepositoryInterface_DeleteCategory_Call{Call: _e.mock.On("DeleteCategory", ctx, organizationId, categoryId)}
}

func (_c *MockCategoryRepositoryInterface_DeleteCategory_Call) Run(run func(ctx context.Context, organizationId int, categoryId int)) *MockCategoryRepositoryInterface_DeleteCategory_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int), args[2].(int))
	})
	return _c
}

func (_c *MockCategoryRepositoryInterface_DeleteCategory_Call) Return(_a0 error) *MockCategoryRepositoryInterface_DeleteCategory_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockCategoryRepositoryInterface_DeleteCategory_Call) RunAndReturn(run func(context.Context, int, int) error) *MockCategoryRepositoryInterface_DeleteCategory_Call {
	_c.Call.Return(run)
	return _c
}

// GetCategories provides a mock function with given fields: ctx, organizationId
func (_m *MockCategoryRepositoryInterface) GetCategories(ctx context.Context, organizationId int) ([]models.Category, error) {
	ret := _m.Called(ctx, organizationId)

	if len(ret) == 0 {
		panic("no return value specified for GetCategories")
	}

	var r0 []models.Category
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int) ([]models.Category, error)); ok {
		return rf(ctx, organizationId)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int) []models.Category); ok {
		r0 = rf(ctx, organizationId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]models.Category)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int) error); ok {
		r1 = rf(ctx, organizationId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockCategoryRepositoryInterface_GetCategories_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetCategories'
type MockCategoryRepositoryInterface_GetCategories_Call struct {
	*mock.Call
}

// GetCategories is a helper method to define mock.On call
//   - ctx context.Context
//   - organizationId int
func (_e *MockCategoryRepositoryInterface_Expecter) GetCategories(ctx interface{}, organizationId interface{}) *MockCategoryRepositoryInterface_GetCategories_Call {
	return &MockCategoryRepositoryInterface_GetCategories_Call{Call: _e.mock.On("GetCategories", ctx, organizationId)}
}

func (_c *MockCategoryRepositoryInterface_GetCategories_Call) Run(run func(ctx context.Context, organizationId int)) *MockCategoryRepositoryInterface_GetCategories_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int))
	})
	return _c
}

func (_c *MockCategoryRepositoryInterface_GetCategories_Call) Return(_a0 []models.Category, _a1 error) *MockCategoryRepositoryInterface_GetCategories_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockCategoryRepositoryInterface_GetCategories_Call) RunAndReturn(run func(context.Context, int) ([]models.Category, error)) *MockCategoryRepositoryInterface_GetCategories_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateCategory provides a mock function with given fields: ctx, organizationId, categoryId, _a3
func (_m *MockCategoryRepositoryInterface) UpdateCategory(ctx context.Context, organizationId int, categoryId int, _a3 models.UpdateCategory) error {
	ret := _m.Called(ctx, organizationId, categoryId, _a3)

	if len(ret) == 0 {
		panic("no return value specified for UpdateCategory")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int, int, models.UpdateCategory) error); ok {
		r0 = rf(ctx, organizationId, categoryId, _a3)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockCategoryRepositoryInterface_UpdateCategory_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateCategory'
type MockCategoryRepositoryInterface_UpdateCategory_Call struct {
	*mock.Call
}

// UpdateCategory is a helper method to define mock.On call
//   - ctx context.Context
//   - organizationId int
//   - categoryId int
//   - _a3 models.UpdateCategory
func (_e *MockCategoryRepositoryInterface_Expecter) UpdateCategory(ctx interface{}, organizationId interface{}, categoryId interface{}, _a3 interface{}) *MockCategoryRepositoryInterface_UpdateCategory_Call {
	return &MockCategoryRepositoryInterface_UpdateCategory_Call{Call: _e.mock.On("UpdateCategory", ctx, organizationId, categoryId, _a3)}
}

func (_c *MockCategoryRepositoryInterface_UpdateCategory_Call) Run(run func(ctx context.Context, organizationId int, categoryId int, _a3 models.UpdateCategory)) *MockCategoryRepositoryInterface_UpdateCategory_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int), args[2].(int), args[3].(models.UpdateCategory))
	})
	return _c
}

func (_c *MockCategoryRepositoryInterface_UpdateCategory_Call) Return(_a0 error) *MockCategoryRepositoryInterface_UpdateCategory_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockCategoryRepositoryInterface_UpdateCategory_Call) RunAndReturn(run func(context.Context, int, int, models.UpdateCategory) error) *MockCategoryRepositoryInterface_UpdateCategory_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockCategoryRepositoryInterface creates a new instance of MockCategoryRepositoryInterface. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockCategoryRepositoryInterface(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockCategoryRepositoryInterface {
	mock := &MockCategoryRepositoryInterface{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
