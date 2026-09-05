package saga

import "github.com/google/uuid"

// newUUID returns a new random UUID string.
func newUUID() string {
	return uuid.New().String()
}
