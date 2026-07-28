package schemaregistry

import (
	"context"

	avro "github.com/iskorotkov/avro/v2"
	"github.com/twmb/franz-go/pkg/sr"
)

// Config is the connection configuration for the schema registry client.
// Mirrors libs/kafka.Config's shape and purpose: plain data for the common
// case (today just the URL — auth fields get added here once a non-local
// registry's scheme is decided), so cmd/main.go never needs to import
// franz-go/pkg/sr directly for ordinary construction.
type Config struct {
	URL string
}

// NewClient constructs a schema registry HTTP client from cfg. extraOpts
// lets a caller layer additional sr.ClientOpt values (e.g. a one-off
// sr.DialTLSConfig) on top — this package only ever builds the connection,
// never decides what a caller does with it, same division of concerns as
// libs/kafka.NewClient.
func NewClient(cfg Config, extraOpts ...sr.ClientOpt) (*sr.Client, error) {
	opts := append([]sr.ClientOpt{sr.URLs(cfg.URL)}, extraOpts...)
	return sr.NewClient(opts...)
}

// Register performs an idempotent create-or-return call for subject against
// schema, returning its registry ID: if an identical schema is already
// registered under subject, the registry returns the existing ID rather
// than creating a duplicate. This is the primitive cmd/registerschemas
// (Task 4) uses to walk eventschemas.Schemas() generically — it operates on
// schemas alone, with no Go type involved, unlike RegisterEncode/
// RegisterDecode in serde.go. It is also what RegisterEncode calls to make
// its own schema exist in the registry the first time a producer type is
// registered.
func Register(ctx context.Context, client *sr.Client, subject string, schema avro.Schema) (int, error) {
	ss, err := client.CreateSchema(ctx, subject, sr.Schema{Schema: schema.String(), Type: sr.TypeAvro})
	if err != nil {
		return 0, err
	}
	return ss.ID, nil
}
