package routes

import (
	"context"
	"errors"
	"net/http"

	"github/marveldo/eda-monolith/internal/api/services"
	"github/marveldo/eda-monolith/shared"

	"github.com/go-chi/jwtauth/v5"
)

func (rt *Routes) Authenticator(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, _, err := jwtauth.FromContext(r.Context())
		log := rt.RequestLogger("user.middleware.extract")
		_, span := rt.GetServiceCtx(r.Context(), "user.middleware.extract")
		if span != nil {
			defer span.End()
		}
		if err != nil {
			rt.WriteAppError(w, log, span, &shared.AppError{
				Err:     err,
				Message: "Unauhorized Inavlid or Expired Token",
				Code:    http.StatusUnauthorized,
			})
			return
		}
		if token == nil {
			rt.WriteAppError(w, log, span, &shared.AppError{
				Err:     err,
				Message: "Unauhorized Inavlid or Expired Token",
				Code:    http.StatusUnauthorized,
			})
			return
		}
		next.ServeHTTP(w, r)
	})

}

func (rt *Routes) AuthMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := rt.RequestLogger("user.middleware.extract")
		_, span := rt.GetServiceCtx(r.Context(), "user.middleware.extract")
		if span != nil {
			defer span.End()
		}
		_, claims, err := jwtauth.FromContext(r.Context())
		if err != nil {
			rt.WriteAppError(w, log, span, &shared.AppError{
				Err:     err,
				Message: "Unauhorized Context Mapping",
				Code:    http.StatusUnauthorized,
			})
			return
		}
		tokenType, _ := claims[services.TokenTypeClaim].(string)
		if tokenType != services.TokenTypeAccess {
			rt.WriteAppError(w, log, span, &shared.AppError{
				Err:     errors.New("token is not an access token"),
				Message: "Unauhorized, An Access Token Is Required",
				Code:    http.StatusUnauthorized,
			})
			return
		}
		id, ok := claims["sub"].(string)
		if !ok {
			rt.WriteAppError(w, log, span, &shared.AppError{
				Err:     err,
				Message: "Sub Couldnt be mapped to string",
				Code:    http.StatusUnauthorized,
			})
			return
		}
		ctx := context.WithValue(r.Context(), "user_id", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
