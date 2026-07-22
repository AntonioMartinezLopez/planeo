package kafka

import (
	"crypto/tls"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// Config is the shared connection configuration for any Kafka client this
// app creates. Which SASL/SCRAM mechanism to use and whether to dial with
// TLS are config, not a compile-time choice, so moving from this repo's
// local SASL_PLAINTEXT/SCRAM-SHA-256 setup to a production SASL_SSL/
// SCRAM-SHA-512 broker never requires a code change in a producer or
// consumer - only these fields' values change.
type Config struct {
	Brokers    []string
	Username   string
	Password   string
	Mechanism  string // "SCRAM-SHA-256" or "SCRAM-SHA-512"
	TLSEnabled bool
}

// NewClient builds a franz-go client authenticated per cfg. extraOpts lets
// a caller layer its own options (e.g. inbox's consumer-group settings) on
// top of the shared broker/SASL/TLS configuration - this package only ever
// builds the connection, never decides what a caller does with it.
func NewClient(cfg Config, extraOpts ...kgo.Opt) (*kgo.Client, error) {
	mechanism, err := saslMechanism(cfg.Mechanism, cfg.Username, cfg.Password)
	if err != nil {
		return nil, err
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.SASL(mechanism),
	}
	if cfg.TLSEnabled {
		opts = append(opts, kgo.DialTLSConfig(&tls.Config{}))
	}
	opts = append(opts, extraOpts...)

	return kgo.NewClient(opts...)
}

func saslMechanism(mechanism, username, password string) (sasl.Mechanism, error) {
	auth := scram.Auth{User: username, Pass: password}
	switch mechanism {
	case "SCRAM-SHA-256":
		return auth.AsSha256Mechanism(), nil
	case "SCRAM-SHA-512":
		return auth.AsSha512Mechanism(), nil
	default:
		return nil, fmt.Errorf("unsupported KAFKA_SASL_MECHANISM %q: must be SCRAM-SHA-256 or SCRAM-SHA-512", mechanism)
	}
}
