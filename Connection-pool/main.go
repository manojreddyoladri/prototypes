package main

import (
	"database/sql"
	"sync"
	"time"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// Connection string for MySQL running in Docker container
// Format: "user:password@tcp(host:port)/dbname"
const dsn = "root:password@tcp(localhost:3307)/prototypes"

// newConn opens a single new MySQL connection using the DSN.
// Used when creating the pool (NewCPool) and in the non-pooled benchmark.
func newConn() *sql.DB {
	_db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	return _db
}

// conn wraps a single database connection so the pool can hand it out and take it back.
type conn struct {
	db *sql.DB
}

// cpool holds a fixed set of connections and a channel to lend/return them safely.
// conns: all connections (used by Close to shut them down).
// channel: available connections; Get receives from it, Put sends back to it.
type cpool struct {
	conns []*conn
	mu *sync.Mutex
	channel chan *conn
	maxConn int
}

// NewCPool creates a connection pool with maxConn pre-opened connections.
// Each connection is stored in pool.conns and also sent on pool.channel so
// Get/Put can borrow and return connections via the channel.
func NewCPool(maxConn int) (*cpool, error) {
	var mu = sync.Mutex{}
	pool := &cpool{
		mu: &mu,
		conns: make([]*conn, 0, maxConn),
		channel: make(chan *conn, maxConn),
		maxConn: maxConn,
	}
	for i := 0; i < maxConn; i++ {
		pool.conns = append(pool.conns, &conn{newConn()})
		pool.channel <- pool.conns[i]
	}
	return pool, nil
}

// Close shuts down the pool: closes the channel, then closes every connection
// in pool.conns. Call this when the pool is no longer needed (e.g. at program exit).
func (pool *cpool) Close() {
	close(pool.channel)
	for _, conn := range pool.conns {
		conn.db.Close()
	}
}

// Get borrows a connection from the pool by receiving from the channel.
// The caller must call Put(conn) when done so the connection goes back to the pool.
func (pool *cpool) Get() (*conn, error) {
	c := <-pool.channel
	return c, nil
}

// Put returns a borrowed connection to the pool by sending it on the channel.
// After Put, the caller must not use the connection anymore.
func (pool *cpool) Put(c *conn) {
	pool.channel <- c
}

// benchmarkNonPool runs many goroutines that each open their own connection
func benchmarkNonPool() {
	startTime := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 250; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db := newConn()
			_, err := db.Exec("SELECT SLEEP(0.01);")
			if err != nil {
				panic(err)
			}
			db.Close()
		}()
	}
	wg.Wait()
	fmt.Printf("Benchmark Non Connection Pool: %s\n", time.Since(startTime))
}

// benchmarkPool runs many goroutines that each Get a connection, run one query, and Put it back.
func benchmarkPool() {
	startTime := time.Now()
	pool, err := NewCPool(10)
	if err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := pool.Get()
			if err != nil {
				panic(err)
			}
			_, err = conn.db.Exec("SELECT SLEEP(0.01);")
			if err != nil {
				panic(err)
			}
			pool.Put(conn)
		}()
	}
	wg.Wait()
	fmt.Printf("Benchmark Connection Pool: %s\n", time.Since(startTime))
	pool.Close()
}

func main() {
	//benchmarkNonPool()
	benchmarkPool()
}