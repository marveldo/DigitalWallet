package events

import "time"


const (
	EventUserCreated = "user.created"
	EventUserLoggedIn = "user.loggedIn"
)


type UserCreatedPayload struct {
	UserID    string
	Email     string
	FirstName string
	LastName  string
	CreatedAt time.Time
}

type UserLoggedInPayload struct {
	FirstName string
	LastName string
	Email string
	UserID string
}
