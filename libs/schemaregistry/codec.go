package schemaregistry

import (
	"context"

	avro "github.com/iskorotkov/avro/v2"
)

// Encoder adapts the generic Encode[T] function into a plain, single-method
// interface a caller can inject and mock — Go doesn't allow generic methods,
// so Encode[T] itself can never satisfy a plain interface. T must already be
// registered on serde via RegisterEncode before use.
type Encoder[T any] struct {
	serde *Serde
}

func NewEncoder[T any](serde *Serde) *Encoder[T] {
	return &Encoder[T]{serde: serde}
}

func (e *Encoder[T]) Encode(v T) ([]byte, error) {
	return Encode(e.serde, v)
}

// Decoder adapts the generic Decode[T] function the same way, pairing serde
// with the fixed reader schema Decode needs on every call.
type Decoder[T any] struct {
	serde        *Serde
	readerSchema avro.Schema
}

func NewDecoder[T any](serde *Serde, readerSchema avro.Schema) *Decoder[T] {
	return &Decoder[T]{serde: serde, readerSchema: readerSchema}
}

func (d *Decoder[T]) Decode(ctx context.Context, b []byte) (T, error) {
	var v T
	err := Decode(ctx, d.serde, b, d.readerSchema, &v)
	return v, err
}
