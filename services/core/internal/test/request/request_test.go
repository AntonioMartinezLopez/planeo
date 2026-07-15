package request

import (
	"context"
	"fmt"
	"testing"

	jsonHelper "planeo/libs/json"
	"planeo/services/core/internal/domain/request"
	requestDto "planeo/services/core/internal/infra/rest/api/v1/requests"
	"planeo/services/core/internal/test/utils"

	"github.com/stretchr/testify/assert"
)

//nolint:gocyclo
func TestRequestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Start integration environment
	env := utils.NewIntegrationTestEnvironment(t)
	testApi := env.Api

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
					response := testApi.Get("/v1/organizations/1/requests?pageSize=10", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

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
			response := testApi.Get("/v1/organizations/1/requests?pageSize=10", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Get("/v1/organizations/1/requests?pageSize=10")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Get("/v1/organizations/1/requests?pageSize=10", "Authorization: Bearer invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 for unknown organization to user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Get("/v1/organizations/3525/requests?pageSize=10", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should return all requests with pageSize 10", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Get("/v1/organizations/1/requests?pageSize=10", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)
		})

		t.Run("should return paginated requests", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Get("/v1/organizations/1/requests?pageSize=3", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			var requests struct {
				Requests   []request.Request
				NextCursor int
			}
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &requests, true)
			assert.Equal(t, 200, response.Code)
			assert.Equal(t, 3, len(requests.Requests))

			// Fetch the next page
			nextPageResponse := testApi.Get(fmt.Sprintf("/v1/organizations/1/requests?pageSize=3&cursor=%d", requests.NextCursor), fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))
			_ = jsonHelper.DecodeJSONAndValidate(nextPageResponse.Result().Body, &requests, true)
			assert.Equal(t, 200, nextPageResponse.Code)
			assert.Equal(t, 2, len(requests.Requests))
		})

		t.Run("should filter requests by single category", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)

			// Filter by category 1 (Installation)
			response := testApi.Get("/v1/organizations/1/requests?pageSize=10&selectedCategories=1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			var requests struct {
				Requests   []request.Request
				NextCursor int
			}
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &requests, true)
			assert.Equal(t, 200, response.Code)

			// All returned requests should have CategoryId = 1
			for _, req := range requests.Requests {
				assert.Equal(t, 1, *req.CategoryId)
			}
		})

		t.Run("should filter requests by multiple categories", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)

			// Filter by categories 1 and 2 (Installation and Maintenance)
			response := testApi.Get("/v1/organizations/1/requests?pageSize=10&selectedCategories=1&selectedCategories=2", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			var requests struct {
				Requests   []request.Request
				NextCursor int
			}
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &requests, true)
			assert.Equal(t, 200, response.Code)

			// All returned requests should have CategoryId = 1 or 2
			for _, req := range requests.Requests {
				assert.True(t, *req.CategoryId == 1 || *req.CategoryId == 2, "Expected CategoryId to be 1 or 2, got %d", *req.CategoryId)
			}
		})

		t.Run("should return all requests when no category filter is provided", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)

			// No category filter
			response := testApi.Get("/v1/organizations/1/requests?pageSize=10", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			var requests struct {
				Requests   []request.Request
				NextCursor int
			}
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &requests, true)
			assert.Equal(t, 200, response.Code)
			assert.GreaterOrEqual(t, len(requests.Requests), 5, "Expected at least 5 requests without filter")
		})
	})

	t.Run("POST /requests ", func(t *testing.T) {

		t.Run("should return 403 when user is not authorized", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Post("/v1/organizations/1/requests", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), requestDto.CreateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Post("/v1/organizations/1/requests", "", requestDto.CreateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Post("/v1/organizations/1/requests", "Authorization: Bearer invalid", requestDto.CreateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 for unknown organization to user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Post("/v1/organizations/3525/requests", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), requestDto.CreateRequestInputBody{})

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should create a request", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)

			body := requestDto.CreateRequestInputBody{
				Subject:    "Test request",
				Text:       "Test request",
				Name:       "Test request",
				Email:      "test.test@test.com",
				Address:    "123 Main St",
				Telephone:  "123-456-7890",
				Closed:     false,
				CategoryId: 1,
			}

			var responseBody struct {
				Id int `json:"id"`
			}
			response := testApi.Post("/v1/organizations/1/requests", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), body)
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &responseBody, true)

			assert.Equal(t, 201, response.Code)
			assert.Greater(t, responseBody.Id, 0)
		})

		t.Run("should return 400 when required fields are missing or have invalid types", func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					session, err := env.GetUserSession("admin", "admin")

					if err != nil {
						t.Error(err)
					}
					assert.NotNil(t, session)

					response := testApi.Post("/v1/organizations/1/requests", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), tc.body)
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
			response := testApi.Put("/v1/organizations/1/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), requestDto.UpdateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Put("/v1/organizations/1/requests/1", "", requestDto.UpdateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Put("/v1/organizations/1/requests/1", "Authorization: Bearer invalid", requestDto.UpdateRequestInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 for unknown organization to user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Put("/v1/organizations/3525/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), requestDto.UpdateRequestInputBody{})

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should update a request", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)

			body := requestDto.UpdateRequestInputBody{
				Subject:    "Updated request",
				Text:       "Updated request",
				Name:       "Updated name",
				Email:      "updated.email@test.com",
				Address:    "456 Updated St",
				Telephone:  "987-654-3210",
				Closed:     true,
				CategoryId: 1,
			}

			response := testApi.Put("/v1/organizations/1/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), body)
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

					response := testApi.Put("/v1/organizations/1/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), tc.body)
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
			response := testApi.Delete("/v1/organizations/1/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Delete("/v1/organizations/1/requests/1", "")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Delete("/v1/organizations/1/requests/1", "Authorization: Bearer invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 for unknown organization to user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)
			response := testApi.Delete("/v1/organizations/3525/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should delete a request", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}
			assert.NotNil(t, session)

			response := testApi.Delete("/v1/organizations/1/requests/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))
			assert.Equal(t, 204, response.Code)
		})
	})
}

func TestCreateRequestIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)

	t.Run("creating a request twice with the same organization+referenceId returns the same id", func(t *testing.T) {
		newRequest := request.NewRequest{
			Subject:        "Idempotency test",
			Text:           "body",
			Email:          "sender@example.com",
			OrganizationId: 1,
			ReferenceId:    "duplicate-message-id",
		}

		firstId, err := env.DB.CreateRequest(context.Background(), newRequest)
		assert.Nil(t, err)
		assert.NotZero(t, firstId)

		secondId, err := env.DB.CreateRequest(context.Background(), newRequest)
		assert.Nil(t, err)
		assert.Equal(t, firstId, secondId, "reprocessing the same source email must resolve to the same Request row, not create a duplicate")
	})

	t.Run("requests without a referenceId remain unconstrained", func(t *testing.T) {
		manualRequest := request.NewRequest{
			Subject:        "Manually created",
			Text:           "body",
			Email:          "operator@example.com",
			OrganizationId: 1,
			ReferenceId:    "",
		}

		firstId, err := env.DB.CreateRequest(context.Background(), manualRequest)
		assert.Nil(t, err)

		secondId, err := env.DB.CreateRequest(context.Background(), manualRequest)
		assert.Nil(t, err)
		assert.NotEqual(t, firstId, secondId, "requests with an empty referenceId (e.g. created manually) must not be deduplicated")
	})
}

func TestCreateAndUpdateRequestParticipateInTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)

	t.Run("CreateRequest and UpdateRequest both commit together inside WithTransaction", func(t *testing.T) {
		var requestId int
		err := env.DB.WithTransaction(context.Background(), func(ctx context.Context) error {
			id, err := env.DB.CreateRequest(ctx, request.NewRequest{
				Subject:        "Tx test",
				Text:           "body",
				Email:          "tx@example.com",
				OrganizationId: 1,
				ReferenceId:    "tx-participation-test",
			})
			if err != nil {
				return err
			}
			requestId = id
			return env.DB.UpdateRequest(ctx, request.UpdateRequest{
				Id:             id,
				Text:           "updated body",
				Subject:        "Tx test updated",
				Email:          "tx@example.com",
				OrganizationId: 1,
			})
		})
		assert.Nil(t, err)

		got, err := env.DB.GetRequest(context.Background(), 1, requestId)
		assert.Nil(t, err)
		assert.Equal(t, "updated body", got.Text, "both writes must be visible after a successful transaction")
	})

	t.Run("a failure after CreateRequest rolls back the whole transaction", func(t *testing.T) {
		err := env.DB.WithTransaction(context.Background(), func(ctx context.Context) error {
			_, err := env.DB.CreateRequest(ctx, request.NewRequest{
				Subject:        "Rollback test",
				Text:           "body",
				Email:          "rollback@example.com",
				OrganizationId: 1,
				ReferenceId:    "tx-rollback-test",
			})
			if err != nil {
				return err
			}
			// A nonexistent CategoryId violates the requests.category_id
			// foreign key, forcing a real transaction-aborting error.
			return env.DB.UpdateRequest(ctx, request.UpdateRequest{
				Id:             999999,
				OrganizationId: 1,
				CategoryId:     999999,
			})
		})
		assert.NotNil(t, err, "the forced foreign key violation must surface as an error")

		requests, err := env.DB.GetRequests(context.Background(), 1, 0, 100, false, nil)
		assert.Nil(t, err)
		for _, r := range requests {
			assert.NotEqual(t, "tx-rollback-test", r.ReferenceId, "CreateRequest's row must not survive when the transaction as a whole fails")
		}
	})
}
