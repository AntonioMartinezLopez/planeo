package huma_utils

import "github.com/danielgtaylor/huma/v2"

func WithAuth(operation huma.Operation) huma.Operation {
	operation.Security = []map[string][]string{{"bearer": {}}}
	return operation
}
