package main

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"airline-checkin/airline"
	dbio "airline-checkin/io"
)

var errNoSeat = errors.New("no seat available")

func book(user *airline.User) error {
	txn, err := dbio.DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = txn.Rollback()
	}()

	var seatID int
	row := txn.QueryRow("SELECT id FROM seats WHERE trip_id = 1 AND user_id IS NULL ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED")
	if err := row.Scan(&seatID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNoSeat
		}
		return err
	}

	result, err := txn.Exec("UPDATE seats SET user_id = ? WHERE id = ? AND user_id IS NULL", user.ID, seatID)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errNoSeat
	}

	if err := txn.Commit(); err != nil {
		return err
	}

	return nil
}

func assignedCount() int {
	var count int
	_ = dbio.DB.QueryRow("SELECT COUNT(*) FROM seats WHERE trip_id = 1 AND user_id IS NOT NULL").Scan(&count)
	return count
}

func main() {
	if err := dbio.Init(); err != nil {
		panic(err)
	}

	airline.Reset()
	users := airline.Users()

	var successes int64
	var noSeat int64
	var failures int64

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(len(users))
	for ix := range users {
		go func(user *airline.User) {
			defer wg.Done()
			err := book(user)
			switch {
			case err == nil:
				atomic.AddInt64(&successes, 1)
			case errors.Is(err, errNoSeat):
				atomic.AddInt64(&noSeat, 1)
			default:
				atomic.AddInt64(&failures, 1)
			}
		}(&users[ix])
	}
	wg.Wait()
	elapsed := time.Since(start)

	fmt.Printf("RESULT approach=3 users=%d success=%d no_seat=%d failures=%d assigned=%d duration_ms=%d\n",
		len(users), successes, noSeat, failures, assignedCount(), elapsed.Milliseconds())
}