package request

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"planeo/services/core/pkg/logger"
	"time"
)

var backoffSchedule = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	10 * time.Second,
}

func sendRequestWithRetry(request *http.Request) (*http.Response, error) {
	client := &http.Client{}
	var response *http.Response
	var error error

	for _, backoff := range backoffSchedule {
		response, error = client.Do(request)

		if error == nil {
			break
		}

		logger.Error("Request error: %v, Retrying in %v", error, backoff)
		time.Sleep(backoff)
	}

	// All retries failed
	if error != nil {
		return nil, error
	}

	return response, nil
}

type HttpMethod string

const (
	GET    HttpMethod = "GET"
	POST   HttpMethod = "POST"
	PUT    HttpMethod = "PUT"
	PATCH  HttpMethod = "PATCH"
	DELETE HttpMethod = "DELETE"
)

type ContentType string

const (
	ApplicationJSON           ContentType = "application/json"
	ApplicationFormURLEncoded ContentType = "application/x-www-form-urlencoded"
)

type HttpRequestParams struct {
	Method      HttpMethod
	URL         string
	Headers     map[string]string
	QueryParams map[string]string
	Body        interface{}
	ContentType ContentType
}

func HttpRequestWithRetry(params HttpRequestParams) (*http.Response, error) {

	// 1. Construct the URL with query parameters if any
	reqURL, err := url.Parse(params.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Add query parameters to the URL
	if len(params.QueryParams) > 0 {
		q := reqURL.Query()
		for key, value := range params.QueryParams {
			q.Add(key, value)
		}
		reqURL.RawQuery = q.Encode()
	}

	// 2. Prepare the request body based on the content type
	var requestBody []byte
	switch params.ContentType {
	case ApplicationJSON:
		// Handle JSON encoding
		if params.Body != nil {
			jsonData, err := json.Marshal(params.Body)
			if err != nil {
				return nil, fmt.Errorf("error encoding JSON: %w", err)
			}
			requestBody = jsonData
		}
	case ApplicationFormURLEncoded:
		// Handle URL encoding
		if bodyMap, ok := params.Body.(map[string]string); ok {
			formData := url.Values{}
			for key, value := range bodyMap {
				formData.Set(key, value)
			}
			requestBody = []byte(formData.Encode())
		} else {
			return nil, fmt.Errorf("body must be of type map[string]string for form encoding")
		}
	default:
		return nil, fmt.Errorf("unsupported content type: %s", params.ContentType)
	}

	request, err := http.NewRequest(string(params.Method), reqURL.String(), bytes.NewBuffer(requestBody))

	if err != nil {
		logger.Error("Error creating request: %v", err)
		return nil, err
	}

	// 3. Set the headers
	for k, v := range params.Headers {
		request.Header.Set(k, v)
	}

	// 4. Set the Content-Type header
	if params.ContentType != "" {
		request.Header.Set("Content-Type", string(params.ContentType))
	}

	return sendRequestWithRetry(request)
}
