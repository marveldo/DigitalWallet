package repository

import (
	"log/slog"

	"github/marveldo/eda-monolith/internal/repository/db"

	"gorm.io/gorm/clause"
)

type UserRepository struct{}

type UserRepositoryConfig struct{}

func NewUserRepo(repoConig *UserRepositoryConfig) UserRepository {
	return UserRepository{}
}

func (r *UserRepository) CreateUser(ctx *RepoCtx, user *UserInputParam) (*User, error) {
	ctx, span := ctx.Start("user.create")
	defer span.End()
	conn := ctx.DB
	userModel := db.User{
		FirstName:       *user.FirstName,
		LastName:        *user.LastName,
		Email:           *user.Email,
		Passwordhash:    user.Passwordhash,
		PhoneNumber:     user.PhoneNumber,
		DateOfBirth:     user.DateOfBirth,
		ProfilePhotoURL: user.ProfilePhotoURL,
	}
	err := conn.WithContext(ctx.Context).Create(&userModel).Error
	if err != nil {
		return nil, ctx.LogError("user.create", err)
	}
	return r.MapUserModelToUser(&userModel), nil

}

func (r *UserRepository) GetUserByEmail(ctx *RepoCtx, email string) (*User, error) {
	ctx, span := ctx.Start("user.get_by_email")
	defer span.End()
	conn := ctx.DB
	var userModel db.User
	err := conn.WithContext(ctx.Context).Where("email = ?", email).First(&userModel).Error
	if err != nil {
		return nil, ctx.LogError("user.get_by_email", err)
	}
	return r.MapUserModelToUser(&userModel), nil
}

func (r *UserRepository) GetUserByID(ctx *RepoCtx, id string) (*User, error) {
	ctx, span := ctx.Start("user.get_by_id")
	defer span.End()
	conn := ctx.DB
	var userModel db.User
	userID, err := ParseUUID(id)
	if err != nil {
		return nil, ctx.LogError("user.get_by_id", err, slog.String("id", id))
	}
	err = conn.WithContext(ctx.Context).Where("id = ?", userID).First(&userModel).Error
	if err != nil {
		return nil, ctx.LogError("user.get_by_id", err)
	}
	return r.MapUserModelToUser(&userModel), nil
}

func (r *UserRepository) UpdateUser(ctx *RepoCtx, user *UserInputParam) (*User, error) {
	ctx, span := ctx.Start("user.update")
	defer span.End()

	fields := r.UpdateFields(user)

	query := ctx.DB.WithContext(ctx.Context).Clauses(clause.Returning{})
	switch {
	case user.ID != nil:
		userID, err := ParseUUID(*user.ID)
		if err != nil {
			return nil, ctx.LogError("user.update", err, slog.String("id", *user.ID))
		}
		query = query.Where("id = ?", userID)
	case user.Email != nil:
		query = query.Where("email = ?", *user.Email)
		delete(fields, "email")
	default:
		return nil, ctx.LogError("user.update", ErrNoIdentifier)
	}

	if len(fields) == 0 {
		return nil, ctx.LogError("user.update", ErrNothingToUpdate)
	}

	var updated db.User
	result := query.Model(&updated).Updates(fields)
	if result.Error != nil {
		return nil, ctx.LogError("user.update", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ctx.LogError("user.update", ErrNotFound)
	}
	return r.MapUserModelToUser(&updated), nil
}

func (r *UserRepository) UserEmailExists(ctx *RepoCtx, email string) (bool, error) {
	ctx, span := ctx.Start("user.email_exists")
	defer span.End()
	conn := ctx.DB
	var exists bool

	err := conn.WithContext(ctx.Context).Model(db.User{}).Select("count(1) > 0").Where("email = ?", email).Find(&exists).Error

	if err != nil {
		return false, ctx.LogError("user.email_exists", err)
	}

	return exists, nil
}
func (r *UserRepository) DeleteUser(ctx *RepoCtx, id string) error {
	ctx, span := ctx.Start("user.delete")
	defer span.End()
	conn := ctx.DB
	var userModel db.User
	userID, err := ParseUUID(id)
	if err != nil {
		return ctx.LogError("user.delete", err, slog.String("id", id))
	}
	err = conn.WithContext(ctx.Context).Where("id = ?", userID).First(&userModel).Error
	if err != nil {
		return ctx.LogError("user.delete", err)
	}
	userModel.IsActive = false
	err = conn.WithContext(ctx.Context).Save(&userModel).Error
	if err != nil {
		return ctx.LogError("user.delete", err)
	}
	return nil

}

func setIf[T any](fields map[string]any, column string, value *T) {
	if value != nil {
		fields[column] = *value
	}
}

func (r *UserRepository) UpdateFields(user *UserInputParam) map[string]any {
	fields := make(map[string]any, 8)
	setIf(fields, "first_name", user.FirstName)
	setIf(fields, "last_name", user.LastName)
	setIf(fields, "email", user.Email)
	setIf(fields, "phone_number", user.PhoneNumber)
	setIf(fields, "date_of_birth", user.DateOfBirth)
	setIf(fields, "profile_photo_url", user.ProfilePhotoURL)
	setIf(fields, "gender", user.Gender)
	setIf(fields, "is_verified", user.IsVerified)
	return fields
}

func (r *UserRepository) GetAllUsers(ctx *RepoCtx, filters *UserFilters) ([]*User, error) {
	ctx, span := ctx.Start("user.get_all")
	defer span.End()
	conn := ctx.DB
	var userModels []db.User
	var finalusers []*User
	query := conn.WithContext(ctx.Context).Model(&db.User{})
	if filters.FirstName != nil {
		query = query.Where("first_name = ?", *filters.FirstName)
	}
	if filters.LastName != nil {
		query = query.Where("last_name = ?", *filters.LastName)
	}
	if filters.Email != nil {
		query = query.Where("email = ?", *filters.Email)
	}
	if filters.Gender != nil {
		query = query.Where("gender = ?", *filters.Gender)
	}
	err := query.Find(&userModels).Error
	if err != nil {
		return nil, ctx.LogError("user.get_all", err)
	}
	for _, userModel := range userModels {
		finalusers = append(finalusers, r.MapUserModelToUser(&userModel))
	}
	return finalusers, nil

}

func (r *UserRepository) MapUserModelToUser(userModel *db.User) *User {
	return &User{
		ID:              userModel.ID.String(),
		FirstName:       userModel.FirstName,
		LastName:        userModel.LastName,
		Email:           userModel.Email,
		PhoneNumber:     userModel.PhoneNumber,
		DateOfBirth:     userModel.DateOfBirth,
		ProfilePhotoURL: userModel.ProfilePhotoURL,
		Gender:          string(userModel.Gender),
		IsVerified:      userModel.IsVerified,
	}
}
