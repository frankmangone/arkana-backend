package tests

import (
	"database/sql"
	"testing"
)

func TestWalletServiceGetOrCreate(t *testing.T) {
	db := setupTestDB(t)
	svc := newWalletService(db)

	t.Run("creates a new wallet when none exists", func(t *testing.T) {
		w, err := svc.GetOrCreate("0xABC123", "evm")
		if err != nil {
			t.Fatal(err)
		}
		if w.Address != "0xabc123" {
			t.Errorf("address = %q, want lowercased %q", w.Address, "0xabc123")
		}
		if w.System != "evm" {
			t.Errorf("system = %q, want %q", w.System, "evm")
		}
		if w.ID == 0 {
			t.Error("expected a nonzero id")
		}
	})

	t.Run("returns the existing wallet on a repeat call, case-insensitively", func(t *testing.T) {
		first, err := svc.GetOrCreate("0xDEF456", "evm")
		if err != nil {
			t.Fatal(err)
		}

		second, err := svc.GetOrCreate("0xdef456", "evm")
		if err != nil {
			t.Fatal(err)
		}

		if second.ID != first.ID {
			t.Errorf("second.ID = %d, want %d (same wallet, not a duplicate)", second.ID, first.ID)
		}
	})
}

func TestWalletServiceGetByAddress(t *testing.T) {
	db := setupTestDB(t)
	svc := newWalletService(db)

	created, err := svc.GetOrCreate("0xFEED", "evm")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("finds an existing wallet, case-insensitively", func(t *testing.T) {
		w, err := svc.GetByAddress("0xfeed")
		if err != nil {
			t.Fatal(err)
		}
		if w.ID != created.ID {
			t.Errorf("ID = %d, want %d", w.ID, created.ID)
		}
	})

	t.Run("returns sql.ErrNoRows for an unknown address", func(t *testing.T) {
		_, err := svc.GetByAddress("0xnonexistent")
		if err != sql.ErrNoRows {
			t.Errorf("err = %v, want sql.ErrNoRows", err)
		}
	})
}

func TestWalletServiceGetByID(t *testing.T) {
	db := setupTestDB(t)
	svc := newWalletService(db)

	created, err := svc.GetOrCreate("0xBEEF", "evm")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("finds an existing wallet by id", func(t *testing.T) {
		w, err := svc.GetByID(created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if w.Address != "0xbeef" {
			t.Errorf("address = %q, want %q", w.Address, "0xbeef")
		}
	})

	t.Run("returns sql.ErrNoRows for an unknown id", func(t *testing.T) {
		_, err := svc.GetByID(999999)
		if err != sql.ErrNoRows {
			t.Errorf("err = %v, want sql.ErrNoRows", err)
		}
	})
}
