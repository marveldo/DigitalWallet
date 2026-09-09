package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	ErrNotFound           = errors.New("record not found")
	ErrAlreadyExists      = errors.New("record already exists")
	ErrPrerequisiteNotMet = errors.New("prerequisite relation does not exist")
	ErrHasDependents      = errors.New("dependent records exist")
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