package request

import (
	"bytes"
	"net/http"
	"net/url"
	"planeo/api/pkg/logger"
	"time"
)

var backoffSchedule = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	10 * time.Second,
}

type HttpMethod string

const (
	GET    HttpMethod = "GET"
	POST   HttpMethod = "POST"
	UPDATE HttpMethod = "UPDATE"
	PATCH  HttpMethod = "PATCH"
	DELETE HttpMethod = "DELETE"
)

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

func HttpRequestWithRetry(method HttpMethod, url string, payload url.Values, headers map[string]string) (*http.Response, error) {
	request, err := http.NewRequest(string(method), url, bytes.NewBufferString(payload.Encode()))

	if err != nil {
		logger.Error("Error creating request: %v", err)
		return nil, err
	}

	for k, v := range headers {
		request.Header.Set(k, v)
	}

	return sendRequestWithRetry(request)
}
