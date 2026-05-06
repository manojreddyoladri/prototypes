package airline

import "database/sql"

// User represents a passenger trying to check in.
type User struct {
	ID   int
	Name string
}

// Seat represents a seat on a given trip.
type Seat struct {
	ID     int
	Name   string
	TripID int
	UserID sql.NullInt64
}
