package llm

import (
	"context"
	"encoding/json"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/mistral"
)

type ExtractorOutput struct {
	Address string
	Name    string
	Phone   string
}

func ExtractRequestFields(ctx context.Context, requestText string) (ExtractorOutput, error) {

	llm, err := mistral.New(mistral.WithModel("mistral-small-latest"))
	if err != nil {
		return ExtractorOutput{}, err
	}

	messageHistory := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem,
			`
		You are a helpful assistant that extracts fields from requests.
		You will be given a request.
		Please use provided tool "extract_request_fields" to extract the fields from the request.
		If some information needed for tool using can not be found within given text, use empty string.
		`),
		llms.TextParts(llms.ChatMessageTypeHuman,
			`
		Please extract the following fields from the request:
		`+requestText+`
		`),
	}

	// Call the LLM
	response, err := llm.GenerateContent(ctx, messageHistory, llms.WithTools(extraction_tools))
	if err != nil {
		return ExtractorOutput{}, err
	}

	// Check if the response is a function call
	lastMessage := response.Choices[len(response.Choices)-1]

	if len(lastMessage.ToolCalls) > 0 {
		// Get the tool call
		functionCall := lastMessage.ToolCalls[0]

		// Check if the function name is "classify_request"
		if functionCall.FunctionCall.Name == "extract_request_fields" {
			// Get the parameters
			arguments := functionCall.FunctionCall.Arguments

			// parse the arguments string to struct
			output := ExtractorOutput{}

			if err := json.Unmarshal([]byte(arguments), &output); err != nil {
				return ExtractorOutput{}, err
			}

			return output, nil
		}
	}
	return ExtractorOutput{}, nil
}

var extraction_tools = []llms.Tool{
	{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "extract_request_fields",
			Description: "This tool is used to pass fields that were successfully extracted from the request.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address": map[string]any{
						"type":        "string",
						"description": "Postal/physical address of the requester",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Name of the requester",
					},
					"phone": map[string]any{
						"type":        "string",
						"description": "Phone number of the requester",
					},
				},
			},
		},
	},
}
