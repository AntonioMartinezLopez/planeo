package kafka

import "testing"

func TestSaslMechanism(t *testing.T) {
	t.Run("SCRAM-SHA-256 succeeds", func(t *testing.T) {
		if _, err := saslMechanism("SCRAM-SHA-256", "user", "pass"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("SCRAM-SHA-512 succeeds", func(t *testing.T) {
		if _, err := saslMechanism("SCRAM-SHA-512", "user", "pass"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unsupported mechanism errors", func(t *testing.T) {
		if _, err := saslMechanism("PLAIN", "user", "pass"); err == nil {
			t.Fatal("expected an error for an unsupported mechanism, got nil")
		}
	})
}
