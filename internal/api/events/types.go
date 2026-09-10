package events

import "time"


const (
	EventUserCreated = "user.created"
)


type UserCreatedPayload struct {
	UserID    string
	Email     string
	FirstName string
	LastName  string
	CreatedAt time.Time
}
