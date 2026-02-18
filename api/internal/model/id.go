package model

import "github.com/oklog/ulid/v2"

// NewID generates a new ULID string for use as an entity identifier.
func NewID() string {
	return ulid.Make().String()
}
