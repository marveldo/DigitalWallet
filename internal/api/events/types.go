package events

import "time"

// Event names. These are the contract between publisher and subscriber —
// renaming one silently unsubscribes every listener, since a name with no
// handlers is not an error.
const (
	EventUserCreated = "user.created"
)

// UserCreatedPayload is published once a user row exists. It carries the ids
// and the few fields a listener needs to act, not the whole entity: a listener
// that needs more should read it back through the repository.
type UserCreatedPayload struct {
	UserID    string
	Email     string
	FirstName string
	LastName  string
	CreatedAt time.Time
}
