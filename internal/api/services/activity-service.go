package services

import (
	"errors"
	"log/slog"
	"net/http"

	"github/marveldo/eda-monolith/internal/repository"
	"github/marveldo/eda-monolith/shared"
)

// Activity pagination bounds. A caller may ask for fewer than MaxActivityLimit
// rows but never more, so a missing or absurd limit cannot turn into a full
// table scan.
const (
	DefaultActivityLimit = 20
	MaxActivityLimit     = 100
)

// CallerID reads the authenticated user's id that AuthMiddleWare put on the
// request context. It returns an error rather than asserting, so a handler
// mounted outside the auth group fails as a 401 instead of panicking.
func CallerID(ctx *ServiceCtx) (string, *shared.AppError) {
	id, ok := ctx.Value("user_id").(string)
	if !ok || id == "" {
		return "", &shared.AppError{
			Err:     errors.New("no user_id on request context"),
			Message: "Unauthorized",
			Code:    http.StatusUnauthorized,
		}
	}
	return id, nil
}

func (s *Service) MapActivityToServiceDomain(activity *repository.Activity) Activity {
	return Activity{
		ID:        activity.Id,
		UserID:    activity.UserID,
		Action:    activity.Action,
		CreatedAt: activity.CreatedAt,
	}
}

// GetMyActivities returns the authenticated caller's activity feed. The user
// id comes from the token, never from the request, so a caller can only ever
// read their own history.
func (s *Service) GetMyActivities(ctx *ServiceCtx, page *ActivityPageParam) (*ActivityList, *shared.AppError) {
	ctx, span := ctx.Start("activity.list.me")
	defer span.End()

	userID, appErr := CallerID(ctx)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	log := ctx.Logger.With(slog.String("service", "activity.list.me"), slog.String("user_id", userID))

	limit, offset := page.Normalise()

	repo_ctx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})
	result, err := s.Repository.GetUserActivities(repo_ctx, userID, limit, offset)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			log.Warn("activity requested for an unknown user")
			return nil, ctx.Fail(&shared.AppError{Message: "User Not Found", Err: err, Code: http.StatusNotFound})
		}
		log.Error("could not load activities", slog.Any("error", err))
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Activities", Err: err, Code: http.StatusInternalServerError})
	}

	activities := make([]Activity, 0, len(result.Activities))
	for _, activity := range result.Activities {
		activities = append(activities, s.MapActivityToServiceDomain(activity))
	}

	log.Info("activities listed", slog.Int("count", len(activities)), slog.Int64("total", result.Total))
	return &ActivityList{
		Activities: activities,
		Total:      result.Total,
		Limit:      limit,
		Offset:     offset,
	}, nil
}
