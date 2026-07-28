package events

import (
	"context"
	"encoding/json"
	"fmt"

	"planeo/libs/eventschemas"
	"planeo/libs/schemaregistry"
	domainoutbox "planeo/services/email/internal/domain/outbox"
)

// EmailReceivedProducer wraps the real Kafka producer so the JSON-staged
// outbox payload gets Avro-encoded right before it reaches Kafka — this is
// the coupling site described in the design spec's Section 3:
// domain/outbox.Service.ProcessRecord stays completely unaware this
// conversion happens. serde is already registered for EmailReceived by the
// time it reaches here (see cmd/main.go, Step 6) — this package never
// imports franz-go/pkg/sr, and this constructor is plain, matching every
// other infra adapter constructor in this codebase.
//
// Note: services/email/internal/infra/outbox.EmailReceivedProducer already
// exists (the driving adapter owning the polling loop, calling
// domain/outbox.Service.ProcessRecord) — this is a deliberate near-duplicate
// name, a different type in a different package (infra/events, not
// infra/outbox), disambiguated only by package qualification.
type EmailReceivedProducer struct {
	inner domainoutbox.Producer
	serde *schemaregistry.Serde
}

func NewEmailReceivedProducer(inner domainoutbox.Producer, serde *schemaregistry.Serde) *EmailReceivedProducer {
	return &EmailReceivedProducer{inner: inner, serde: serde}
}

func (p *EmailReceivedProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	var wire eventschemas.EmailReceived
	if err := json.Unmarshal(value, &wire); err != nil {
		return fmt.Errorf("decode staged outbox payload: %w", err)
	}

	avroBytes, err := schemaregistry.Encode(p.serde, wire)
	if err != nil {
		return fmt.Errorf("avro encode: %w", err)
	}

	return p.inner.ProduceSync(ctx, topic, key, avroBytes)
}
