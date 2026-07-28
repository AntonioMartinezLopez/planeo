package main

import (
	"context"
	"fmt"
	"os"

	"planeo/libs/eventschemas"
	"planeo/libs/schemaregistry"
)

func main() {
	registryURL := os.Getenv("SCHEMA_REGISTRY_URL")
	if registryURL == "" {
		fmt.Fprintln(os.Stderr, "SCHEMA_REGISTRY_URL is required")
		os.Exit(1)
	}

	client, err := schemaregistry.NewClient(schemaregistry.Config{URL: registryURL})
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating schema registry client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	for subject, schema := range eventschemas.Schemas() {
		id, err := schemaregistry.Register(ctx, client, subject, schema)
		if err != nil {
			fmt.Fprintf(os.Stderr, "registering subject %q: %v\n", subject, err)
			os.Exit(1)
		}
		fmt.Printf("registered subject %q as schema id %d\n", subject, id)
	}
}
