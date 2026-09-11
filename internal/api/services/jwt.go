package services

import (
	"errors"
	"github/marveldo/eda-monolith/shared"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/jwtauth/v5"
)

var token *jwtauth.JWTAuth

// TokenTypeClaim distinguishes the two tokens login issues. Both are signed
// with the same key, so without it a refresh token would be accepted as a
// bearer token on every protected route, and an access token would be enough
// to mint new ones forever.
const TokenTypeClaim = "token_type"

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type JWTConfig struct {
	SecretKey string
}

func NewJwt(jwt *JWTConfig) *jwtauth.JWTAuth {
	secretKey := []byte(jwt.SecretKey)
	tokenAuth := jwtauth.New("HS256", secretKey, nil)
	return tokenAuth
}

type ClaimsPayoad struct {
    ID  string
	Email string
	FirstName string
	LastName  string
	IsVerified bool
}
func (s *Service) GenerateAccessToken(ctx *ServiceCtx , payload *ClaimsPayoad) (*string, *shared.AppError) {
    ctx , span := ctx.Start("access_token.generate")
	defer span.End()
    
	log := s.Logger.With(slog.String("service", "accesstoken.generate"), slog.String("user", payload.ID))
	claims := map[string]interface{}{
		"sub": payload.ID,
		"email": payload.Email,
		"first_name": payload.FirstName,
		"last_name" : payload.LastName,
		"is_verified" : payload.IsVerified,
		TokenTypeClaim : TokenTypeAccess,
	}

	jwtauth.SetExpiry(claims , time.Now().Add(time.Hour * time.Duration(s.AccessTokenExpiry)))

	_ , tokenString , err := s.JWTAuth.Encode(claims)
	if err != nil {
        log.Error("could not generate access token", slog.Any("error", err))
		return nil , ctx.Fail(&shared.AppError{ Err: err , Message: "Failed To generate Access token", Code: http.StatusInternalServerError})
	}
	return shared.Ptr(tokenString), nil
}

func (s *Service) GenerateRefreshToken(ctx *ServiceCtx , payload *ClaimsPayoad) (*string, *shared.AppError) {
    ctx , span := ctx.Start("refreshtoken.generate")
	defer span.End()
    
	log := s.Logger.With(slog.String("service", "refreshtoken.generate"), slog.String("user", payload.ID))
	claims := map[string]interface{}{
		"sub": payload.ID,
		"email": payload.Email,
		"first_name": payload.FirstName,
		"last_name" : payload.LastName,
		"is_verified" : payload.IsVerified,
		TokenTypeClaim : TokenTypeRefresh,
	}

	jwtauth.SetExpiry(claims , time.Now().Add(time.Hour * time.Duration(s.RefreshTokenExpiry)))

	_ , tokenString , err := s.JWTAuth.Encode(claims)
	if err != nil {
        log.Error("could not generate refresh token", slog.Any("error", err))
		return nil , ctx.Fail(&shared.AppError{ Err: err , Message: "Failed To generate Refresh token", Code: http.StatusInternalServerError})
	}
	return shared.Ptr(tokenString), nil
}

// ParseRefreshToken checks a refresh token's signature and expiry and returns
// the user id it was issued for. An access token presented here is rejected.
func (s *Service) ParseRefreshToken(ctx *ServiceCtx , tokenString string) (string, *shared.AppError) {
    ctx , span := ctx.Start("refreshtoken.parse")
	defer span.End()

	log := s.Logger.With(slog.String("service", "refreshtoken.parse"))
	unauthorized := func(err error) *shared.AppError {
		return ctx.Fail(&shared.AppError{ Err: err , Message: "Invalid Or Expired Refresh Token", Code: http.StatusUnauthorized})
	}

	token , err := jwtauth.VerifyToken(s.JWTAuth , tokenString)
	if err != nil {
		log.Warn("refresh token rejected", slog.Any("error", err))
		return "" , unauthorized(err)
	}

	var tokenType string
	if err := token.Get(TokenTypeClaim , &tokenType); err != nil || tokenType != TokenTypeRefresh {
		log.Warn("token presented to refresh is not a refresh token", slog.String("token_type", tokenType))
		return "" , unauthorized(errors.New("token is not a refresh token"))
	}

	subject , ok := token.Subject()
	if !ok || subject == "" {
		log.Warn("refresh token carries no subject")
		return "" , unauthorized(errors.New("refresh token has no subject"))
	}
	return subject , nil
}
