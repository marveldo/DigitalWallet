package routes

import (
	"log/slog"
	"net/http"
	"strconv"

	"github/marveldo/eda-monolith/internal/api/services"
)

// GetMyActivities serves GET /api/v1/me/activity. The account is taken from
// the access token, so there is no id in the path to tamper with.
func (rt *Routes) GetMyActivities(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("activity.list.me")
	serviceCtx, span := rt.GetServiceCtx(r.Context(), "activity.list.me")
	if span != nil {
		defer span.End()
	}

	limit, err := rt.QueryInt(r, "limit")
	if err != nil {
		rt.WriteError(w, log, span, http.StatusBadRequest, "Invalid limit", err)
		return
	}
	offset, err := rt.QueryInt(r, "offset")
	if err != nil {
		rt.WriteError(w, log, span, http.StatusBadRequest, "Invalid offset", err)
		return
	}

	result, appErr := rt.Services.GetMyActivities(serviceCtx, &services.ActivityPageParam{
		Limit:  limit,
		Offset: offset,
	})
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	activities := make([]ActivityResponse, 0, len(result.Activities))
	for _, activity := range result.Activities {
		activities = append(activities, rt.MapActivityToRouteDomain(activity))
	}

	log.Info("activities returned", slog.Int("count", len(activities)))
	rt.WriteJSON(w, http.StatusOK, ActivityListResponse{
		Activities: activities,
		Total:      result.Total,
		Limit:      result.Limit,
		Offset:     result.Offset,
	})
}

// QueryInt reads an optional integer query parameter. An absent parameter is
// not an error; a present but unparseable one is.
func (rt *Routes) QueryInt(r *http.Request, name string) (*int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (rt *Routes) MapActivityToRouteDomain(activity services.Activity) ActivityResponse {
	return ActivityResponse{
		ID:        activity.ID,
		UserID:    activity.UserID,
		Action:    activity.Action,
		CreatedAt: activity.CreatedAt,
	}
}
