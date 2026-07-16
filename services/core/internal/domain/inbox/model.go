package inbox

// ExtractorOutput holds the fields the LLM extracted from a request.
type ExtractorOutput struct {
	Address string
	Name    string
	Phone   string
}

// RequestData is the LLM classifier's input shape for a single request.
type RequestData struct {
	Subject string
	Text    string
}
