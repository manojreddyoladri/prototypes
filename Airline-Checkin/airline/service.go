package airline

import (
	"fmt"

	dbio "airline-checkin/io"
	log "github.com/sirupsen/logrus"
)

// Users returns all users from the database.
func Users() []User {
	if err := dbio.Init(); err != nil {
		log.Errorf("failed to initialize database: %v", err)
		return nil
	}

	rows, err := dbio.DB.Query("SELECT id, name FROM users ORDER BY id")
	if err != nil {
		log.Errorf("failed to query users: %v", err)
		return nil
	}
	defer rows.Close()

	users := make([]User, 0, 128)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name); err != nil {
			log.Errorf("failed to scan user row: %v", err)
			continue
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		log.Errorf("users rows iteration failed: %v", err)
	}

	return users
}

// PrintSeats prints current seat assignment snapshot.
func PrintSeats() {
	if err := dbio.Init(); err != nil {
		log.Errorf("failed to initialize database: %v", err)
		return
	}

	query := `
SELECT s.name, COALESCE(u.name, 'UNASSIGNED')
FROM seats s
LEFT JOIN users u ON s.user_id = u.id
WHERE s.trip_id = 1
ORDER BY s.id
`
	rows, err := dbio.DB.Query(query)
	if err != nil {
		log.Errorf("failed to query seats: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var seatName string
		var userName string
		if err := rows.Scan(&seatName, &userName); err != nil {
			log.Errorf("failed to scan seat row: %v", err)
			continue
		}
		fmt.Printf("%s -> %s\n", seatName, userName)
	}

	if err := rows.Err(); err != nil {
		log.Errorf("seats rows iteration failed: %v", err)
	}
}
