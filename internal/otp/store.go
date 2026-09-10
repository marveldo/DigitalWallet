package otp

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github/marveldo/eda-monolith/config"
	"github/marveldo/eda-monolith/shared"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrNotFound     = errors.New("otp not found or expired")
	ErrMismatch     = errors.New("otp does not match")
	ErrTooManyTries = errors.New("too many otp attempts")
)

type Store struct {
	Client      *redis.Client
	Logger      *slog.Logger
	Tracer      trace.Tracer
	Length      int
	TTL         time.Duration
	MaxAttempts int
}

type StoreConfig struct {
	Client *redis.Client
	Logger *slog.Logger
	Tracer trace.Tracer
	Config config.OTPConfig
}

func NewRedisClient(cfg config.AsynqBackgroundWorker) (*redis.Client, error) {
	opts, err := redis.ParseURL(cfg.RedisUrl)
	if err != nil {
		return nil, fmt.Errorf("parsing otp redis url: %w", err)
	}
	if cfg.Username != "" {
		opts.Username = cfg.Username
	}
	if cfg.Password != "" {
		opts.Password = cfg.Password
	}
	opts.DB = cfg.DB
	return redis.NewClient(opts), nil
}

func NewStore(cfg *StoreConfig) *Store {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	length := cfg.Config.Length
	if length <= 0 {
		length = shared.DefaultOTPLength
	}
	ttl := cfg.Config.TTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	maxAttempts := cfg.Config.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	return &Store{
		Client:      cfg.Client,
		Logger:      logger.With(slog.String("layer", "otp")),
		Tracer:      cfg.Tracer,
		Length:      length,
		TTL:         ttl,
		MaxAttempts: maxAttempts,
	}
}

func (s *Store) startSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	if s.Tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return s.Tracer.Start(ctx, name)
}

func codeKey(identifier string) string    { return "otp:code:" + identifier }
func attemptKey(identifier string) string { return "otp:attempts:" + identifier }

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func (s *Store) Generate(ctx context.Context, identifier string) (string, error) {
	ctx, span := s.startSpan(ctx, "otp.generate")
	defer span.End()

	code, err := shared.GenerateOTP(s.Length)
	if err != nil {
		s.fail(ctx, span, "could not generate otp", identifier, err)
		return "", err
	}

	pipe := s.Client.TxPipeline()
	pipe.Set(ctx, codeKey(identifier), hashCode(code), s.TTL)
	pipe.Del(ctx, attemptKey(identifier))
	if _, err := pipe.Exec(ctx); err != nil {
		s.fail(ctx, span, "could not store otp", identifier, err)
		return "", fmt.Errorf("storing otp: %w", err)
	}

	s.Logger.InfoContext(ctx, "otp issued",
		slog.String("identifier", identifier),
		slog.Duration("ttl", s.TTL),
	)
	return code, nil
}

func (s *Store) Verify(ctx context.Context, identifier string, code string) error {
	ctx, span := s.startSpan(ctx, "otp.verify")
	defer span.End()

	attempts, err := s.Client.Incr(ctx, attemptKey(identifier)).Result()
	if err != nil {
		s.fail(ctx, span, "could not count otp attempt", identifier, err)
		return fmt.Errorf("counting otp attempt: %w", err)
	}
	if attempts == 1 {
		s.Client.Expire(ctx, attemptKey(identifier), s.TTL)
	}
	if attempts > int64(s.MaxAttempts) {
		s.Client.Del(ctx, codeKey(identifier), attemptKey(identifier))
		s.Logger.WarnContext(ctx, "otp locked out after too many attempts",
			slog.String("identifier", identifier),
			slog.Int64("attempts", attempts),
		)
		return ErrTooManyTries
	}

	stored, err := s.Client.Get(ctx, codeKey(identifier)).Result()
	if errors.Is(err, redis.Nil) {
		return ErrNotFound
	}
	if err != nil {
		s.fail(ctx, span, "could not read otp", identifier, err)
		return fmt.Errorf("reading otp: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(stored), []byte(hashCode(code))) != 1 {
		s.Logger.WarnContext(ctx, "otp mismatch",
			slog.String("identifier", identifier),
			slog.Int64("attempts", attempts),
		)
		return ErrMismatch
	}

	s.Client.Del(ctx, codeKey(identifier), attemptKey(identifier))
	s.Logger.InfoContext(ctx, "otp verified", slog.String("identifier", identifier))
	return nil
}

func (s *Store) fail(ctx context.Context, span trace.Span, msg string, identifier string, err error) {
	s.Logger.ErrorContext(ctx, msg,
		slog.String("identifier", identifier),
		slog.Any("error", err),
	)
	if span != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
