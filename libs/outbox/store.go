package outbox

// Record is a single outbox row ready to be produced to Kafka: already
// fully serialized bytes, opaque to the runner/adapter. The Store
// interface that used to live alongside this type has been removed —
// each service now declares its own Repository interface (see
// services/email/internal/infra/outbox), satisfied directly by that
// service's *postgres.Client. Record itself stays here, shared, so the
// adapter and the repository layer agree on exactly one type.
type Record struct {
	ID      int64
	Topic   string
	Key     []byte
	Payload []byte
}
