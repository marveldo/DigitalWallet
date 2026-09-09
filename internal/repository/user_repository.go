package repository

import "github/marveldo/eda-monolith/internal/repository/db"

type UserRepository struct{}

type UserRepositoryConfig struct{}

func NewUserRepo(repoConig *UserRepositoryConfig) UserRepository {
	return UserRepository{}
}

func (r *UserRepository) CreateUser(ctx *RepoCtx, user *UserInputParam) (*User, error) {
	conn := ctx.DB
	userModel := db.User{
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		Email:           user.Email,
		Passwordhash:    user.Passwordhash,
		PhoneNumber:     user.PhoneNumber,
		DateOfBirth:     user.DateOfBirth,
		ProfilePhotoURL: user.ProfilePhotoURL,
	}
	err := conn.WithContext(ctx.Context).Create(&userModel).Error
	if err != nil {
		return nil, wrapGormError(err)
	}
	return r.MapUserModelToUser(&userModel), nil

}

func (r *UserRepository) GetUserByEmail(ctx *RepoCtx, email string) (*User, error) {
	conn := ctx.DB
	var userModel db.User
	err := conn.WithContext(ctx.Context).Where("email = ?", email).First(&userModel).Error
	if err != nil {
		return nil, wrapGormError(err)
	}
	return r.MapUserModelToUser(&userModel), nil
}

func (r *UserRepository) GetUserByID(ctx *RepoCtx, id string) (*User, error) {
	conn := ctx.DB
	var userModel db.User
	err := conn.WithContext(ctx.Context).Where("id = ?", id).First(&userModel).Error
	if err != nil {
		return nil, wrapGormError(err)
	}
	return r.MapUserModelToUser(&userModel), nil
}

func (r *UserRepository) UpdateUser(ctx *RepoCtx, user *User) (*User, error) {
	conn := ctx.DB
	var userModel db.User
	updateFields := make(map[string]interface{})
	err := conn.WithContext(ctx.Context).Where("id = ?", user.ID).First(&userModel).Error
	if err != nil {
		return nil, wrapGormError(err)
	}

	err = conn.WithContext(ctx.Context).Model(&userModel).Updates(updateFields).Error
	if err != nil {
		return nil, wrapGormError(err)
	}
	r.MapUpdateFields(user, updateFields)

	err = conn.WithContext(ctx.Context).Where("id = ?", user.ID).First(&userModel).Error
	if err != nil {
		return nil, wrapGormError(err)
	}
	return r.MapUserModelToUser(&userModel), nil
}

func (r *UserRepository) UserEmailExists(ctx *RepoCtx , email string) (bool , error) {
	conn := ctx.DB
	var exists bool 

	err := conn.Model(db.User{}).Select("count(1) > 0").Where("email = ?", email).Find(&exists).Error

	if err != nil {
		return false , wrapGormError(err)
	}

	return exists , nil
}
func (r *UserRepository) DeleteUser(ctx *RepoCtx, id string) error {
	conn := ctx.DB
	var userModel db.User
	err := conn.WithContext(ctx.Context).Where("id = ?", id).First(&userModel).Error
	if err != nil {
		return wrapGormError(err)
	}
	userModel.IsActive = false
	err = conn.WithContext(ctx.Context).Save(&userModel).Error
	if err != nil {
		return wrapGormError(err)
	}
	return nil

}

func (r *UserRepository) MapUpdateFields(user *User, updateFields map[string]interface{}) {
	if user.FirstName != "" {
		updateFields["first_name"] = user.FirstName
	}
	if user.LastName != "" {
		updateFields["last_name"] = user.LastName
	}
	if user.Email != "" {
		updateFields["email"] = user.Email
	}
	if user.PhoneNumber != nil {
		updateFields["phone_number"] = user.PhoneNumber
	}
	if user.DateOfBirth != nil {
		updateFields["date_of_birth"] = user.DateOfBirth
	}
	if user.ProfilePhotoURL != nil {
		updateFields["profile_photo_url"] = user.ProfilePhotoURL
	}
	if user.Gender != "" {
		updateFields["gender"] = user.Gender
	}
}

func (r *UserRepository) GetAllUsers(ctx *RepoCtx, filters *UserFilters) ([]*User, error) {
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
		return nil, wrapGormError(err)
	}
	for _, userModel := range userModels {
		finalusers = append(finalusers, r.MapUserModelToUser(&userModel))
	}
	return finalusers, nil

}


func (r *UserRepository) MapUserModelToUser(userModel *db.User) *User {
	return &User{
		ID:              userModel.ID,
		FirstName:       userModel.FirstName,
		LastName:        userModel.LastName,
		Email:           userModel.Email,
		PhoneNumber:     userModel.PhoneNumber,
		DateOfBirth:     userModel.DateOfBirth,
		ProfilePhotoURL: userModel.ProfilePhotoURL,
		Gender:          string(userModel.Gender),
	}
}
