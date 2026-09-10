package otp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github/marveldo/eda-monolith/config"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	cfg := config.AsynqBackgroundWorker{RedisUrl: "redis://localhost:6399"}
	client, err := NewRedisClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("redis not available:", err)
	}
	return NewStore(&StoreConfig{Client: client, Config: config.OTPConfig{
		Length: 6, 
		TTL: 2 * time.Second, 
		MaxAttempts: 3,
	}})
}

func TestGenerateAndVerify(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	code, err := s.Generate(ctx, "ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Fatalf("want 6 digits, got %q", code)
	}

	raw := s.Client.Get(ctx, codeKey("ada@example.com")).Val()
	if raw == code {
		t.Error("otp stored in plaintext")
	}

	if err := s.Verify(ctx, "ada@example.com", code); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := s.Verify(ctx, "ada@example.com", code); !errors.Is(err, ErrNotFound) {
		t.Errorf("reuse should fail with ErrNotFound, got %v", err)
	}
}

func TestWrongCodeAndLockout(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Generate(ctx, "grace@example.com"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := s.Verify(ctx, "grace@example.com", "000000"); !errors.Is(err, ErrMismatch) {
			t.Fatalf("attempt %d: want ErrMismatch, got %v", i, err)
		}
	}
	if err := s.Verify(ctx, "grace@example.com", "000000"); !errors.Is(err, ErrTooManyTries) {
		t.Errorf("want ErrTooManyTries, got %v", err)
	}
}

func TestExpiry(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	code, err := s.Generate(ctx, "alan@example.com")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2500 * time.Millisecond)
	if err := s.Verify(ctx, "alan@example.com", code); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound after ttl, got %v", err)
	}
}
