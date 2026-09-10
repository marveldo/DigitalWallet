package repository

import (
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"gorm.io/gorm"
)

var (
	ErrNotFound           = errors.New("record not found")
	ErrAlreadyExists      = errors.New("record already exists")
	ErrPrerequisiteNotMet = errors.New("prerequisite relation does not exist")
	ErrHasDependents      = errors.New("dependent records exist")
	ErrNothingToUpdate    = errors.New("no fields to update")
	ErrNoIdentifier       = errors.New("update requires an id or email")
	ErrLessonLocked       = errors.New("lesson is locked by the course's progression rules")
)

func wrapGormError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrAlreadyExists
	}
	return err
}

func gormDeletedAtToPtr(dt gorm.DeletedAt) *time.Time {
	if dt.Valid {
		return &dt.Time
	}
	return nil
}

// LogError normalises a gorm error and records it on the context's logger. A
// missing row is an ordinary outcome rather than a fault, so it logs at debug;
// everything else is an error.
func (c *RepoCtx) LogError(op string, err error, attrs ...any) error {
	wrapped := wrapGormError(err)
	if wrapped == nil || c.Logger == nil {
		return wrapped
	}
	args := append([]any{slog.String("repo_op", op), slog.Any("error", wrapped)}, attrs...)
	notFound := errors.Is(wrapped, ErrNotFound)
	if notFound {
		c.Logger.DebugContext(c.Context, "repository record not found", args...)
	} else {
		c.Logger.ErrorContext(c.Context, "repository query failed", args...)
	}

	// Record on this operation's own span, so the trace shows which query
	// failed rather than only that the request did. A missing row is an
	// ordinary outcome, so it is left unrecorded — marking it an error would
	// turn every "does this email exist" check red.
	if c.Span != nil && !notFound {
		c.Span.RecordError(wrapped)
		c.Span.SetStatus(codes.Error, wrapped.Error())
	}
	return wrapped
}

// ParseUUID converts an id from the transport layer into a uuid. A malformed
// id can never match a row, so it is reported as ErrNotFound rather than being
// sent to Postgres to fail there as a 500.
func ParseUUID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, ErrNotFound
	}
	return parsed, nil
}
