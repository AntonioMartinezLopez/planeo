package operations

import "github.com/danielgtaylor/huma/v2"

func WithAuth(operation huma.Operation) huma.Operation {
	operation.Security = []map[string][]string{{"myAuth": {}}}
	return operation
}
