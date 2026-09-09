package services

import (
	"github/marveldo/eda-monolith/internal/repository"
	"github/marveldo/eda-monolith/shared"
	"net/http"
	"time"
)

func (s *Service) MapUserToServiceDomain(user *repository.User) *User {
   return &User{
	FirstName: user.FirstName,
	LastName: user.LastName,
	Email:  user.Email,
	DateOfBirth: user.DateOfBirth,
	PhoneNumber: user.PhoneNumber,
	ProfilePhotoURL:  user.ProfilePhotoURL,
	Gender: user.Gender,
   }
}

func (s *Service) CreateUser(ctx *ServiceCtx, userInput *UserInputParam) (*User, *shared.AppError) {
    repo_ctx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: ctx.Span})
	exists , err := s.Repository.UserEmailExists(repo_ctx , userInput.Email)
	var dob time.Time
	layout := "2006-01-02 15:04:00"
	if err != nil {
       return nil, &shared.AppError{Message:  err.Error() , Err: err, Code: http.StatusInternalServerError}
	}
	if exists {
		return nil , &shared.AppError{Message: "User With This Email Already Exists",Err : err , Code : http.StatusConflict}
	}
    hashed_password , err := HashPassword([]byte(userInput.Password))
	if err != nil {
		return nil , &shared.AppError{Message:  "Error Registering User", Err:err, Code : http.StatusInternalServerError}
	}
	if userInput.DateOfBirth != nil {
		obj , err := time.Parse(layout , *userInput.DateOfBirth)
	    if err != nil {
            return nil , &shared.AppError{Message:  "Error Registering User", Err:err, Code : http.StatusInternalServerError}
		}
		dob = obj
	}
	stringfiedHash := string(hashed_password)
	user , err := s.Repository.CreateUser(repo_ctx , &repository.UserInputParam{
		FirstName: userInput.FirstName,
		Email: userInput.Email,
		PhoneNumber: userInput.PhoneNumber,
		DateOfBirth: &dob ,
		ProfilePhotoURL: userInput.ProfilePhotoURL,
		Gender : userInput.Gender,
		Passwordhash: &stringfiedHash,
	})
	return  s.MapUserToServiceDomain(user) , nil
}



