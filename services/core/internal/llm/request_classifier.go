package llm

import (
	"context"
	"encoding/json"
	"fmt"
	categories "planeo/services/core/internal/resources/category/models"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/mistral"
)

type RequestData struct {
	Subject string
	Text    string
}

func ClassifyRequest(ctx context.Context, request RequestData, categories []categories.Category) (int, error) {

	llm, err := mistral.New(mistral.WithModel("mistral-small-latest"))

	if err != nil {
		return 0, err
	}

	// Build string of request
	requestString := fmt.Sprintf("Subject: %s,\nText: %s\n", request.Subject, request.Text)

	// Build string of categories
	categoriesString := ""
	for i, category := range categories {
		categoriesString += fmt.Sprintf("ID: %d, Label: %s, Description: %s,\n", i, category.Label, category.LabelDescription)
	}

	messageHistory := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem,
			`
		You are a helpful assistant that classifies requests into categories.

		You will be given a request and a list of categories.
		Your task is to classify the request into one of the categories.

		The categories are as follows:

		`+categoriesString+`
		
		Please use provided tool "classify_request" to classify the request
		If none of the categories fit, call the tool with 0 for the request parameter.		
		`),
		llms.TextParts(llms.ChatMessageTypeHuman,
			`
		Please classify the following request:
		`+requestString+`
		`),
	}

	// Call the LLM
	response, err := llm.GenerateContent(ctx, messageHistory, llms.WithTools(classifier_tools))
	if err != nil {
		return 0, err
	}

	// Check if the response is a function call
	lastMessage := response.Choices[len(response.Choices)-1]

	if len(lastMessage.ToolCalls) > 0 {
		// Get the tool call
		functionCall := lastMessage.ToolCalls[0]

		// Check if the function name is "classify_request"
		if functionCall.FunctionCall.Name == "classify_request" {
			// Get the parameters
			arguments := functionCall.FunctionCall.Arguments

			// parse the arguments string to struct
			var args struct {
				CategoryId int `json:"category_id"`
			}

			if err := json.Unmarshal([]byte(arguments), &args); err != nil {
				return 0, err
			}

			return args.CategoryId, nil
		}
	}
	return 0, nil
}

var classifier_tools = []llms.Tool{
	{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "classify_request",
			Description: "This function classifies a request into a category.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"category_id": map[string]any{
						"type":        "integer",
						"description": "The ID of the category to classify the request into. Set to 0 if none of the categories fit.",
					},
				},
				"required": []string{
					"category_id",
				},
			},
		},
	},
}
