package airline

import (
	log "github.com/sirupsen/logrus"

	dbio "airline-checkin/io"
)

// Reset clears all seat assignments for trip 1.
func Reset() {
	if err := dbio.Init(); err != nil {
		log.Errorf("failed to initialize database: %v", err)
		return
	}

	if _, err := dbio.DB.Exec("UPDATE seats SET user_id = NULL WHERE trip_id = 1"); err != nil {
		log.Errorf("failed to reset seats: %v", err)
	}
}
