package services

import (
	"errors"
	"github/marveldo/eda-monolith/shared"
	"log/slog"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

var cost = 11

func HashPassword(password []byte) ([]byte, error) {

	hashed_password, err := bcrypt.GenerateFromPassword(password, cost)
	if err != nil {
		return nil, err
	}
	return hashed_password, nil
}

func CompareHash(hashedPassword []byte, password []byte) error {
	return bcrypt.CompareHashAndPassword(hashedPassword, password)
}

func (s *Service) ValidatOwner(ctx *ServiceCtx, accountid string, userid string) (*User, *shared.AppError) {
	ctx, span := ctx.Start("validate.user")
	log := s.Logger.With(slog.String("user", "verifyOwnership"), slog.String("id", accountid))
	defer span.End()
	repo_ctx := s.GetRepoCtx(ServiceCtxConfig{
		Context: ctx,
		Span:    span,
		Logger:  log,
	})
	if accountid != userid {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("UserId and Account Id dont match"),
			Code:    http.StatusForbidden,
			Message: "User cant delete another account",
		})
	}
	user, err := s.Repository.GetUserByID(repo_ctx, accountid)
	if err != nil {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("Account Id not found"),
			Code:    http.StatusForbidden,
			Message: "Account Doesnt Exist",
		})
	}
	return s.MapUserToServiceDomain(user), nil

}
