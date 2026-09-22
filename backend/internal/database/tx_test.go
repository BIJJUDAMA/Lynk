package database

import (
	"context"
	"errors"
	"testing"
)

type mockTx struct {
	committed   bool
	rolledBack  bool
	commitErr   error
	rollbackErr error
}

func (m *mockTx) Commit(ctx context.Context) error {
	m.committed = true
	return m.commitErr
}

func (m *mockTx) Rollback(ctx context.Context) error {
	m.rolledBack = true
	return m.rollbackErr
}

func TestWithTxMock_CommitOnSuccess(t *testing.T) {
	mtx := &mockTx{}
	ctx := context.Background()

	err := RunTx(ctx, mtx, func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !mtx.committed {
		t.Errorf("expected tx to be committed")
	}
	if mtx.rolledBack {
		t.Errorf("expected tx NOT to be rolled back")
	}
}

func TestWithTxMock_RollbackOnError(t *testing.T) {
	mtx := &mockTx{}
	ctx := context.Background()
	expectedErr := errors.New("business logic failed")

	err := RunTx(ctx, mtx, func(ctx context.Context) error {
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got: %v", expectedErr, err)
	}
	if mtx.committed {
		t.Errorf("expected tx NOT to be committed")
	}
	if !mtx.rolledBack {
		t.Errorf("expected tx to be rolled back")
	}
}

func TestWithTxMock_RollbackOnPanic(t *testing.T) {
	mtx := &mockTx{}
	ctx := context.Background()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic to propagate")
		}
		if !mtx.rolledBack {
			t.Errorf("expected tx to be rolled back on panic")
		}
		if mtx.committed {
			t.Errorf("expected tx NOT to be committed on panic")
		}
	}()

	_ = RunTx(ctx, mtx, func(ctx context.Context) error {
		panic("fatal unexpected panic")
	})
}

func TestWithTxMock_CommitFailure(t *testing.T) {
	commitErr := errors.New("commit disk full")
	mtx := &mockTx{commitErr: commitErr}
	ctx := context.Background()

	err := RunTx(ctx, mtx, func(ctx context.Context) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected commit error, got nil")
	}
	if !errors.Is(err, commitErr) {
		t.Fatalf("expected wrapped commit error %v, got %v", commitErr, err)
	}
	if !mtx.committed {
		t.Errorf("expected commit to be attempted")
	}
}
