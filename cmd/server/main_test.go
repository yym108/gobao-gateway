package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForMySQLSuccessAfterRetry(t *testing.T) {
	t.Parallel()

	attempts := 0
	err := waitForMySQL(context.Background(), 5, time.Millisecond, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("connect: connection refused")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("waitForMySQL returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("waitForMySQL attempts = %d, want 3", attempts)
	}
}

func TestWaitForMySQLReturnsLastErrorAfterExhausted(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("connect: connection refused")
	attempts := 0
	err := waitForMySQL(context.Background(), 3, time.Millisecond, func(context.Context) error {
		attempts++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("waitForMySQL error = %v, want %v", err, wantErr)
	}
	if attempts != 3 {
		t.Fatalf("waitForMySQL attempts = %d, want 3", attempts)
	}
}
