// Code generated by mockery. DO NOT EDIT.

package mocks

import (
	dto "planeo/api/internal/resources/user/dto"

	mock "github.com/stretchr/testify/mock"

	models "planeo/api/internal/resources/user/models"
)

// MockIAMInterface is an autogenerated mock type for the IAMInterface type
type MockIAMInterface struct {
	mock.Mock
}

type MockIAMInterface_Expecter struct {
	mock *mock.Mock
}

func (_m *MockIAMInterface) EXPECT() *MockIAMInterface_Expecter {
	return &MockIAMInterface_Expecter{mock: &_m.Mock}
}

// AssignRolesToUser provides a mock function with given fields: organizationId, userId, roles
func (_m *MockIAMInterface) AssignRolesToUser(organizationId string, userId string, roles []dto.PutUserRoleInputBody) error {
	ret := _m.Called(organizationId, userId, roles)

	if len(ret) == 0 {
		panic("no return value specified for AssignRolesToUser")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, []dto.PutUserRoleInputBody) error); ok {
		r0 = rf(organizationId, userId, roles)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockIAMInterface_AssignRolesToUser_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'AssignRolesToUser'
type MockIAMInterface_AssignRolesToUser_Call struct {
	*mock.Call
}

// AssignRolesToUser is a helper method to define mock.On call
//   - organizationId string
//   - userId string
//   - roles []dto.PutUserRoleInputBody
func (_e *MockIAMInterface_Expecter) AssignRolesToUser(organizationId interface{}, userId interface{}, roles interface{}) *MockIAMInterface_AssignRolesToUser_Call {
	return &MockIAMInterface_AssignRolesToUser_Call{Call: _e.mock.On("AssignRolesToUser", organizationId, userId, roles)}
}

func (_c *MockIAMInterface_AssignRolesToUser_Call) Run(run func(organizationId string, userId string, roles []dto.PutUserRoleInputBody)) *MockIAMInterface_AssignRolesToUser_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(string), args[2].([]dto.PutUserRoleInputBody))
	})
	return _c
}

func (_c *MockIAMInterface_AssignRolesToUser_Call) Return(_a0 error) *MockIAMInterface_AssignRolesToUser_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockIAMInterface_AssignRolesToUser_Call) RunAndReturn(run func(string, string, []dto.PutUserRoleInputBody) error) *MockIAMInterface_AssignRolesToUser_Call {
	_c.Call.Return(run)
	return _c
}

// CreateUser provides a mock function with given fields: organizationId, createUserInput
func (_m *MockIAMInterface) CreateUser(organizationId string, createUserInput dto.CreateUserInputBody) (*models.User, error) {
	ret := _m.Called(organizationId, createUserInput)

	if len(ret) == 0 {
		panic("no return value specified for CreateUser")
	}

	var r0 *models.User
	var r1 error
	if rf, ok := ret.Get(0).(func(string, dto.CreateUserInputBody) (*models.User, error)); ok {
		return rf(organizationId, createUserInput)
	}
	if rf, ok := ret.Get(0).(func(string, dto.CreateUserInputBody) *models.User); ok {
		r0 = rf(organizationId, createUserInput)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*models.User)
		}
	}

	if rf, ok := ret.Get(1).(func(string, dto.CreateUserInputBody) error); ok {
		r1 = rf(organizationId, createUserInput)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockIAMInterface_CreateUser_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateUser'
type MockIAMInterface_CreateUser_Call struct {
	*mock.Call
}

// CreateUser is a helper method to define mock.On call
//   - organizationId string
//   - createUserInput dto.CreateUserInputBody
func (_e *MockIAMInterface_Expecter) CreateUser(organizationId interface{}, createUserInput interface{}) *MockIAMInterface_CreateUser_Call {
	return &MockIAMInterface_CreateUser_Call{Call: _e.mock.On("CreateUser", organizationId, createUserInput)}
}

func (_c *MockIAMInterface_CreateUser_Call) Run(run func(organizationId string, createUserInput dto.CreateUserInputBody)) *MockIAMInterface_CreateUser_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(dto.CreateUserInputBody))
	})
	return _c
}

func (_c *MockIAMInterface_CreateUser_Call) Return(_a0 *models.User, _a1 error) *MockIAMInterface_CreateUser_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockIAMInterface_CreateUser_Call) RunAndReturn(run func(string, dto.CreateUserInputBody) (*models.User, error)) *MockIAMInterface_CreateUser_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteUser provides a mock function with given fields: organizationId, userId
func (_m *MockIAMInterface) DeleteUser(organizationId string, userId string) error {
	ret := _m.Called(organizationId, userId)

	if len(ret) == 0 {
		panic("no return value specified for DeleteUser")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string) error); ok {
		r0 = rf(organizationId, userId)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockIAMInterface_DeleteUser_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteUser'
type MockIAMInterface_DeleteUser_Call struct {
	*mock.Call
}

// DeleteUser is a helper method to define mock.On call
//   - organizationId string
//   - userId string
func (_e *MockIAMInterface_Expecter) DeleteUser(organizationId interface{}, userId interface{}) *MockIAMInterface_DeleteUser_Call {
	return &MockIAMInterface_DeleteUser_Call{Call: _e.mock.On("DeleteUser", organizationId, userId)}
}

func (_c *MockIAMInterface_DeleteUser_Call) Run(run func(organizationId string, userId string)) *MockIAMInterface_DeleteUser_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(string))
	})
	return _c
}

func (_c *MockIAMInterface_DeleteUser_Call) Return(_a0 error) *MockIAMInterface_DeleteUser_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockIAMInterface_DeleteUser_Call) RunAndReturn(run func(string, string) error) *MockIAMInterface_DeleteUser_Call {
	_c.Call.Return(run)
	return _c
}

// GetRoles provides a mock function with no fields
func (_m *MockIAMInterface) GetRoles() ([]models.Role, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetRoles")
	}

	var r0 []models.Role
	var r1 error
	if rf, ok := ret.Get(0).(func() ([]models.Role, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() []models.Role); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]models.Role)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockIAMInterface_GetRoles_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRoles'
type MockIAMInterface_GetRoles_Call struct {
	*mock.Call
}

// GetRoles is a helper method to define mock.On call
func (_e *MockIAMInterface_Expecter) GetRoles() *MockIAMInterface_GetRoles_Call {
	return &MockIAMInterface_GetRoles_Call{Call: _e.mock.On("GetRoles")}
}

func (_c *MockIAMInterface_GetRoles_Call) Run(run func()) *MockIAMInterface_GetRoles_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockIAMInterface_GetRoles_Call) Return(_a0 []models.Role, _a1 error) *MockIAMInterface_GetRoles_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockIAMInterface_GetRoles_Call) RunAndReturn(run func() ([]models.Role, error)) *MockIAMInterface_GetRoles_Call {
	_c.Call.Return(run)
	return _c
}

// GetUserById provides a mock function with given fields: organizationId, userId
func (_m *MockIAMInterface) GetUserById(organizationId string, userId string) (*models.UserWithRoles, error) {
	ret := _m.Called(organizationId, userId)

	if len(ret) == 0 {
		panic("no return value specified for GetUserById")
	}

	var r0 *models.UserWithRoles
	var r1 error
	if rf, ok := ret.Get(0).(func(string, string) (*models.UserWithRoles, error)); ok {
		return rf(organizationId, userId)
	}
	if rf, ok := ret.Get(0).(func(string, string) *models.UserWithRoles); ok {
		r0 = rf(organizationId, userId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*models.UserWithRoles)
		}
	}

	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(organizationId, userId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockIAMInterface_GetUserById_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetUserById'
type MockIAMInterface_GetUserById_Call struct {
	*mock.Call
}

// GetUserById is a helper method to define mock.On call
//   - organizationId string
//   - userId string
func (_e *MockIAMInterface_Expecter) GetUserById(organizationId interface{}, userId interface{}) *MockIAMInterface_GetUserById_Call {
	return &MockIAMInterface_GetUserById_Call{Call: _e.mock.On("GetUserById", organizationId, userId)}
}

func (_c *MockIAMInterface_GetUserById_Call) Run(run func(organizationId string, userId string)) *MockIAMInterface_GetUserById_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(string))
	})
	return _c
}

func (_c *MockIAMInterface_GetUserById_Call) Return(_a0 *models.UserWithRoles, _a1 error) *MockIAMInterface_GetUserById_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockIAMInterface_GetUserById_Call) RunAndReturn(run func(string, string) (*models.UserWithRoles, error)) *MockIAMInterface_GetUserById_Call {
	_c.Call.Return(run)
	return _c
}

// GetUsers provides a mock function with given fields: organizationId
func (_m *MockIAMInterface) GetUsers(organizationId string) ([]models.User, error) {
	ret := _m.Called(organizationId)

	if len(ret) == 0 {
		panic("no return value specified for GetUsers")
	}

	var r0 []models.User
	var r1 error
	if rf, ok := ret.Get(0).(func(string) ([]models.User, error)); ok {
		return rf(organizationId)
	}
	if rf, ok := ret.Get(0).(func(string) []models.User); ok {
		r0 = rf(organizationId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]models.User)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(organizationId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockIAMInterface_GetUsers_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetUsers'
type MockIAMInterface_GetUsers_Call struct {
	*mock.Call
}

// GetUsers is a helper method to define mock.On call
//   - organizationId string
func (_e *MockIAMInterface_Expecter) GetUsers(organizationId interface{}) *MockIAMInterface_GetUsers_Call {
	return &MockIAMInterface_GetUsers_Call{Call: _e.mock.On("GetUsers", organizationId)}
}

func (_c *MockIAMInterface_GetUsers_Call) Run(run func(organizationId string)) *MockIAMInterface_GetUsers_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockIAMInterface_GetUsers_Call) Return(_a0 []models.User, _a1 error) *MockIAMInterface_GetUsers_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockIAMInterface_GetUsers_Call) RunAndReturn(run func(string) ([]models.User, error)) *MockIAMInterface_GetUsers_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateUser provides a mock function with given fields: organizationId, userId, updateUserInput
func (_m *MockIAMInterface) UpdateUser(organizationId string, userId string, updateUserInput dto.UpdateUserInputBody) error {
	ret := _m.Called(organizationId, userId, updateUserInput)

	if len(ret) == 0 {
		panic("no return value specified for UpdateUser")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, dto.UpdateUserInputBody) error); ok {
		r0 = rf(organizationId, userId, updateUserInput)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockIAMInterface_UpdateUser_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateUser'
type MockIAMInterface_UpdateUser_Call struct {
	*mock.Call
}

// UpdateUser is a helper method to define mock.On call
//   - organizationId string
//   - userId string
//   - updateUserInput dto.UpdateUserInputBody
func (_e *MockIAMInterface_Expecter) UpdateUser(organizationId interface{}, userId interface{}, updateUserInput interface{}) *MockIAMInterface_UpdateUser_Call {
	return &MockIAMInterface_UpdateUser_Call{Call: _e.mock.On("UpdateUser", organizationId, userId, updateUserInput)}
}

func (_c *MockIAMInterface_UpdateUser_Call) Run(run func(organizationId string, userId string, updateUserInput dto.UpdateUserInputBody)) *MockIAMInterface_UpdateUser_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(string), args[2].(dto.UpdateUserInputBody))
	})
	return _c
}

func (_c *MockIAMInterface_UpdateUser_Call) Return(_a0 error) *MockIAMInterface_UpdateUser_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockIAMInterface_UpdateUser_Call) RunAndReturn(run func(string, string, dto.UpdateUserInputBody) error) *MockIAMInterface_UpdateUser_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockIAMInterface creates a new instance of MockIAMInterface. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockIAMInterface(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockIAMInterface {
	mock := &MockIAMInterface{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
