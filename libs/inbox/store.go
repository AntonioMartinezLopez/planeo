package inbox

// Record is one durably-persisted inbox row, ready for processing. The
// Store interface that used to live alongside this type has been removed
// — services/core now declares its own Repository interface (see
// services/core/internal/infra/inbox), satisfied directly by
// *postgres.Client. Record itself stays here, shared, so the adapter and
// the repository layer agree on exactly one type.
type Record struct {
	ID      int64
	Topic   string
	Payload []byte
}
