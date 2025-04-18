package test

import (
	"context"
	"fmt"
	"planeo/libs/api"
	"planeo/libs/db"
	jsonHelper "planeo/libs/json"
	"planeo/libs/middlewares"
	"planeo/services/core/internal/resources/organization"
	"planeo/services/core/internal/resources/request"
	"planeo/services/core/internal/resources/request/dto"
	"planeo/services/core/internal/resources/request/models"
	"planeo/services/core/internal/test/utils"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
)

func TestRequestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Start integration environment
	env := utils.NewIntegrationTestEnvironment(t)
	db := db.InitializeDatabaseConnection(context.Background(), env.Configuration.DatabaseConfig())
	_, testApi := humatest.New(t)

	// setup request controller
	requestRepository := request.NewRequestRepository(db.DB)
	requestService := request.NewRequestService(requestRepository)
	requestController := request.NewRequestController(testApi, env.Configuration, requestService)

	// Register controllers
	jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", env.Configuration.OauthIssuerUrl())
	middlewares := []api.Middleware{
		middlewares.AuthMiddleware(testApi, jwksURL, env.Configuration.OauthIssuerUrl()),
		middlewares.OrganizationCheckMiddleware(testApi, func(organizationId string) (string, error) {
			return organization.GetOrganizationIamById(db.DB, organizationId)
		}),
	}
	controllers := []api.Controller{
		requestController,
	}

	api.InitializeControllers(testApi, middlewares, controllers)

	// table tests for creating and updating requests
	testCases := []struct {
		name string
		body string
	}{
		{
			"missing Text field",
			`{
				"Subject": "Test request",
				"Name": "Test request",
				"Email": "test.test@test.com",
				"Address": "123 Main St",
				"Telephone": "123-456-7890",
				"Closed": false,
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"missing Name field",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Email": "test.test@test.com",
				"Address": "123 Main St",
				"Telephone": "123-456-7890",
				"Closed": false,
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"missing Email field",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Name": "Test request",
				"Address": "123 Main St",
				"Telephone": "123-456-7890",
				"Closed": false,
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"missing Address field",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Name": "Test request",
				"Email": "test.test@test.com",
				"Telephone": "123-456-7890",
				"Closed": false,
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"missing Telephone field",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Name": "Test request",
				"Email": "test.test@test.com",
				"Address": "123 Main St",
				"Closed": false,
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"missing Closed field",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Name": "Test request",
				"Email": "",
				"Address": "123 Main St",
				"Telephone": "123-456-7890",
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"invalid Text field type",
			`{
				"Subject": "Test request",
				"Text": 123,
				"Name": "Test request",
				"Email": "test.test@test.com",
				"Address": "123 Main St",
				"Telephone": "123-456-7890",
				"Closed": false,
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"invalid Name field type",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Name": 123,
				"Email": "test.test@test.com",
				"Address": "123 Main St",
				"Telephone": "123-456-7890",
				"Closed": false,
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"invalid Email field type",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Name": "Test request",
				"Email": 123,
				"Address": "123 Main St",
				"Telephone": "123-456-7890",
				"Closed": false,
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"invalid Address field type",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Name": "Test request",
				"Email": "test.test@test.com",
				"Address": 123,
				"Telephone": "123-456-7890",
				"Closed": false,
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"invalid Telephone field type",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Name": "Test request",
				"Email": "test.test@test.com",
				"Address": "123 Main St",
				"Telephone": 123,
				"Closed": false,
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"invalid Closed field type",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Name": "Test request",
				"Email": "",
				"Address": "123 Main St",
				"Telephone": "123-456-7890",
				"Closed": 123,
				"CategoryId": 1,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"invalid CategoryId field type",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Name": "Test request",
				"Email": "",
				"Address": "123 Main St",
				"Telephone": "123-456-7890",
				"Closed": false,
				"CategoryId": "invalid",
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
		{
			"invalid CategoryId field value",
			`{
				"Subject": "Test request",
				"Text": "Test request",
				"Name": "Test request",
				"Email": "",
				"Address": "123 Main St",
				"Telephone": "123-456-7890",
				"Closed": false,
				"CategoryId": 0,
				"Raw": "Additional raw data",
				"ReferenceId": "REF12345"
			}`,
		},
	}

	t.Run("GET /requests ", func(t *testing.T) {

		t.Run("Test authorization for request access", func(t *testing.T) {
			testCases := []struct {
				name     string
				username string
				password string
			}{
				{"should return 200 when user is admin", "admin", "admin"},
				{"should return 200 when user is planner", "planner", "planner"},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					session, err := env.GetUserSession(tc.username, tc.password)

					if err != nil {
						t.Error(err)
					}

					assert.NotNil(t, session)
					response := testApi.Get("/organizations/1/requests?pageSize=10", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

					assert.Equal(t, 200, response.Code)
				})
			}
		})

		t.Run("should return 403 when user is not authorized", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Get("/organizations/1/requests?pageSize=10", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Get("/organizations/1/requests?pageSize=10")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Get("/organizations/1/requests?pageSize=10", "Authorization: Bearer invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 for unknown organization to user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Get("/organizations/3525/requests?pageSize=10", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should return all requests with pageSize 10", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Get("/organizations/1/requests?pageSize=10", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)
		})

		t.Run("should return paginated requests", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Get("/organizations/1/requests?pageSize=3", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			var requests struct {
				Requests   []models.Request
				NextCursor int
			}
			jsonHelper.DecodeJSONAndValidate(response.Result().Body, &requests, true)
			assert.Equal(t, 200, response.Code)
			assert.Equal(t, 3, len(requests.Requests))

			// Fetch the next page
			nextPageResponse := testApi.Get(fmt.Sprintf("/organizations/1/requests?pageSize=3&cursor=%d", requests.NextCursor), fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))
			jsonHelper.DecodeJSONAndValidate(nextPageResponse.Result().Body, &requests, true)
			assert.Equal(t, 200, nextPageResponse.Code)
			assert.Equal(t, 2, len(requests.Requests))
		})
	})

	t.Run("POST /requests ", func(t *testing.T) {

		t.Run("should return 403 when user is not authorized", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Post("/organizations/1/requests", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), dto.CreateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Post("/organizations/1/requests", "", dto.CreateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Post("/organizations/1/requests", "Authorization: Bearer invalid", dto.CreateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 for unknown organization to user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Post("/organizations/3525/requests", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), dto.CreateRequestInputBody{})

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should create a request", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)

			body := dto.CreateRequestInputBody{
				Subject:    "Test request",
				Text:       "Test request",
				Name:       "Test request",
				Email:      "test.test@test.com",
				Address:    "123 Main St",
				Telephone:  "123-456-7890",
				Closed:     false,
				CategoryId: 1,
			}

			response := testApi.Post("/organizations/1/requests", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), body)
			assert.Equal(t, 201, response.Code)
		})

		t.Run("should return 400 when required fields are missing or have invalid types", func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					session, err := env.GetUserSession("admin", "admin")

					if err != nil {
						t.Error(err)
					}
					assert.NotNil(t, session)

					response := testApi.Post("/organizations/1/requests", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), tc.body)
					assert.Equal(t, 400, response.Code)
				})
			}
		})
	})

	t.Run("PUT /requests/:id", func(t *testing.T) {

		t.Run("should return 403 when user is not authorized", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Put("/organizations/1/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), dto.UpdateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Put("/organizations/1/requests/1", "", dto.UpdateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Put("/organizations/1/requests/1", "Authorization: Bearer invalid", dto.UpdateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 for unknown organization to user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Put("/organizations/3525/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), dto.UpdateRequestInputBody{})

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should update a request", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)

			body := dto.UpdateRequestInputBody{
				Subject:    "Updated request",
				Text:       "Updated request",
				Name:       "Updated name",
				Email:      "updated.email@test.com",
				Address:    "456 Updated St",
				Telephone:  "987-654-3210",
				Closed:     true,
				CategoryId: 1,
			}

			response := testApi.Put("/organizations/1/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), body)
			assert.Equal(t, 204, response.Code)
		})

		t.Run("should return 400 when required fields are missing or have invalid types", func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					session, err := env.GetUserSession("admin", "admin")

					if err != nil {
						t.Error(err)
					}
					assert.NotNil(t, session)

					response := testApi.Put("/organizations/1/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), tc.body)
					assert.Equal(t, 400, response.Code)
				})
			}
		})
	})

	t.Run("DELETE /requests/:id", func(t *testing.T) {

		t.Run("should return 403 when user is not authorized", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Delete("/organizations/1/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Delete("/organizations/1/requests/1", "")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Delete("/organizations/1/requests/1", "Authorization: Bearer invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 for unknown organization to user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Delete("/organizations/3525/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should delete a request", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)

			response := testApi.Delete("/organizations/1/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))
			assert.Equal(t, 204, response.Code)
		})
	})
}
