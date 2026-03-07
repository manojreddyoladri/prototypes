# Connection Pool Prototype

A Go prototype that demonstrates the performance difference between accessing a database **with** a connection pool and **without** one. It includes a small channel-based connection pool implementation and benchmarks for both approaches.

## Description

This project showcases how connection pooling affects throughput and stability when many concurrent goroutines execute database queries. Without a pool, each goroutine opens and closes its own connection, which leads to high latency, resource usage, and eventually "too many connections" errors. With a fixed-size pool, connections are reused and the same workload completes reliably with predictable latency.

## Motivation

I built this prototype to:

- Learn how connection pools work internally (pre-created connections, Get/Put semantics, cleanup).
- Practice Go (channels, goroutines, mutexes, `database/sql`).
- Measure the performance difference between pooled and non-pooled access.

### Observed Results

**Without connection pool**

| Concurrent goroutines (i) | Result                                 |
| ------------------------- | -------------------------------------- |
| 100                       | ~68 ms                                 |
| 150                       | ~71 ms                                 |
| 200                       | ~144 ms                                |
| 250                       | **Error: 1040 – too many connections** |

**With connection pool**

| Concurrent goroutines (i) | Time    |
| ------------------------- | ------- |
| 100                       | ~144 ms |
| 150                       | ~214 ms |
| 200                       | ~276 ms |
| 250                       | ~344 ms |
| 1000                      | ~1.39 s |

Without a pool, the database hits its connection limit; with a pool, the same server handles 1000 concurrent workers by reusing a small set of connections.

## Features

- **Channel-based connection pool** – Fixed-size pool backed by a buffered channel of `*conn`.
- **Get / Put API** – `Get()` borrows a connection; `Put()` returns it to the pool.
- **Pool lifecycle** – `NewCPool(maxConn)` creates and pre-warms the pool; `Close()` closes all connections.
- **Benchmarks** – `benchmarkNonPool()` (new connection per goroutine) and `benchmarkPool()` (shared pool) for comparison.
- **MySQL in Docker** – Optional Docker Compose setup for a local MySQL instance.

## Tech Stack

- **Language:** [Go](https://go.dev/) (1.21+)
- **Database:** [MySQL 8](https://www.mysql.com/)
- **Go packages:** `database/sql`, `sync`, `time`, `fmt`
- **Driver:** [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
- **Infrastructure:** [Docker](https://www.docker.com/) & Docker Compose (for MySQL)

## Prerequisites

- [Go](https://go.dev/dl/) 1.21 or later
- [Docker](https://docs.docker.com/get-docker/) and Docker Compose (for running MySQL locally)

## Installation

1. **Clone or navigate to the project**

   ```bash
   cd Connection-pool
   ```

2. **Install Go dependencies**

   ```bash
   go mod download
   # or
   go mod tidy
   ```

3. **Start MySQL (Docker)**

   Ensure the `dsn` in `main.go` matches the credentials and port in `docker-compose.yml`, then:

   ```bash
   docker compose up -d
   ```

   Wait until MySQL is ready (e.g. 10–20 seconds). The Compose file exposes MySQL on host port **3307** by default (`3307:3306`).

4. **Configure the connection string (if needed)**

   In `main.go`, the DSN is:

   ```go
   const dsn = "root:manola@tcp(localhost:3307)/prototypes"
   ```

   Adjust user, password, host, port, and database name to match your `docker-compose.yml` or existing MySQL instance.

## Usage

### Run the application

From the `Connection-pool` directory:

```bash
go run main.go
```

The program runs the pooled benchmark by default and prints elapsed time, e.g.:

```text
Benchmark Connection Pool: 344.233166ms
```

### Switch to the non-pooled benchmark

In `main.go`, change `main()` to call the non-pooled benchmark instead:

```go
func main() {
	benchmarkNonPool()
	// benchmarkPool()
}
```

Then run:

```bash
go run main.go
```

Example output:

```text
Benchmark Non Connection Pool: 68.227416ms
```

(With higher concurrency you may see "too many connections" without a pool.)

### Using the pool in your own code

```go
pool, err := NewCPool(10)
if err != nil {
	log.Fatal(err)
}
defer pool.Close()

conn, err := pool.Get()
if err != nil {
	log.Fatal(err)
}
defer pool.Put(conn)

_, err = conn.db.Exec("SELECT 1")
// ...
```

## Project Structure

```text
Connection-pool/
├── main.go           # Pool implementation, benchmarks, and main
├── go.mod
├── go.sum
├── docker-compose.yml
└── README.md
```

## Notes

- This is a learning prototype. For production, use a battle-tested pool such as the one provided by `database/sql` (e.g. `sql.Open` with a driver that supports pooling) or a dedicated library.
- The benchmarks use `SELECT SLEEP(0.01)` to simulate a small amount of DB work; adjust iteration counts and sleep duration to explore different loads.
