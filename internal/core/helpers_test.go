package core

import (
	"path/filepath"
	"testing"

	"github.com/BX-Team/Nexon/internal/node"
	"github.com/BX-Team/Nexon/internal/store"
)

// testService builds a Service backed by a fresh temp database with the default
// seed, using the stub node factory (no live xray).
func testService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := st.SeedDefaults(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st, node.StubFactory)
}
