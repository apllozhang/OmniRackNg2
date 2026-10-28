package main

import "testing"

func TestCloneURLRefusesOtherDestinations(t *testing.T) {
	for _, raw := range []string{
		"", "host=127.0.0.1 dbname=omnirack_integration",
		"postgres://localhost/omnirack_integration", "postgres://10.20.30.103/omnirack_integration",
		"postgres://127.0.0.1/omnirack_replica", "postgres://127.0.0.1/postgres",
		"postgres://127.0.0.1/omnirack_integration?host=10.20.30.103",
		"postgres://127.0.0.1/omnirack_integration?dbname=omnirack_replica",
		"postgres://127.0.0.1/omnirack_integration?options=-csearch_path=other",
		"postgres://127.0.0.1/omnirack_integration?service=production",
		"postgres://127.0.0.1/omnirack_integration?search_path=other",
	} {
		t.Run(raw, func(t *testing.T) {
			if err := cloneURL(raw); err == nil {
				t.Fatal("accepted an unsafe clone destination")
			}
		})
	}
}

func TestCloneURLAcceptsExplicitLoopbackClone(t *testing.T) {
	for _, raw := range []string{
		"postgres://restore_check@127.0.0.1:15499/omnirack_integration?sslmode=disable",
		"postgresql://restore_check@[::1]:15499/omnirack_integration?sslmode=disable",
	} {
		if err := cloneURL(raw); err != nil {
			t.Fatalf("rejected an explicit loopback clone: %v", err)
		}
	}
}

func TestRunRequiresExplicitCloneAcknowledgementBeforeConnection(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://restore_check@127.0.0.1:1/omnirack_integration?sslmode=disable")
	t.Setenv("OMNIRACK_DISPOSABLE_MIGRATION_ACK", "")
	if err := run(); err == nil || err.Error() != "set OMNIRACK_DISPOSABLE_MIGRATION_ACK=owned-restored-clone" {
		t.Fatalf("expected acknowledgement guard before database connection, got %v", err)
	}
}
