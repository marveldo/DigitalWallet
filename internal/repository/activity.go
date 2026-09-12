package repository

import (
	"log/slog"
	"time"

	"github/marveldo/eda-monolith/internal/repository/db"
)

type ActivityRepository struct{}

type ActivityRepositoryConfig struct{}

func NewActivityRepository(*ActivityRepositoryConfig) ActivityRepository {
	return ActivityRepository{}
}

func (a *ActivityRepository) MapActivityModelToActivity(activityModel *db.UserActivity) *Activity {
	return &Activity{
		Id:        activityModel.ID,
		UserID:    activityModel.UserID.String(),
		Action:    activityModel.Action,
		CreatedAt: activityModel.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (a *ActivityRepository) CreateActivity(ctx *RepoCtx, userId string, action string) (*Activity, error) {
	conn := ctx.DB
	parsedId, err := ParseUUID(userId)
	if err != nil {
		return nil, err
	}
	activity := &db.UserActivity{
		UserID: parsedId,
		Action: action,
	}

	err = conn.WithContext(ctx.Context).Create(activity).Error
	if err != nil {
		return nil, err
	}

	return a.MapActivityModelToActivity(activity), nil
}

// ActivityPage is one page of a user's activity feed plus the total number of
// rows behind it, so the caller can render "page 2 of 9" without a second
// round trip.
type ActivityPage struct {
	Activities []*Activity
	Total      int64
}

// GetUserActivities returns the user's activities newest first. Limit and
// offset are applied after the count, which is taken over the whole filtered
// set rather than the page.
func (a *ActivityRepository) GetUserActivities(ctx *RepoCtx, userId string, limit int, offset int) (*ActivityPage, error) {
	ctx, span := ctx.Start("activity.get_by_user")
	defer span.End()

	parsedId, err := ParseUUID(userId)
	if err != nil {
		return nil, ctx.LogError("activity.get_by_user", err, slog.String("user_id", userId))
	}

	query := ctx.DB.WithContext(ctx.Context).Model(&db.UserActivity{}).Where("user_id = ?", parsedId)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, ctx.LogError("activity.get_by_user.count", err)
	}

	var activityModels []db.UserActivity
	err = query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&activityModels).Error
	if err != nil {
		return nil, ctx.LogError("activity.get_by_user", err)
	}

	activities := make([]*Activity, 0, len(activityModels))
	for i := range activityModels {
		activities = append(activities, a.MapActivityModelToActivity(&activityModels[i]))
	}
	return &ActivityPage{Activities: activities, Total: total}, nil
}
