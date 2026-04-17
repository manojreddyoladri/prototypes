# Polling prototype

A Go prototype that demonstrates **short polling** and **long polling** against a MySQL-backed status model. It uses a simulated **EC2-style deployment** (async job that updates server status in the database) so you can compare how each polling style behaves when waiting for a long-running operation to finish.

## Description

This prototype shows two HTTP patterns:

- **Short polling** – The client asks once; the server returns the current status immediately. The client must call again on a timer if it wants updates.
- **Long polling** – The server keeps the request open and checks the database until a condition is met (here: until the stored status matches a value the client supplied), then responds. That reduces empty round-trips compared with naive short polling, though it still differs from **WebSockets**, which push events over a persistent connection.

### When long polling instead of WebSockets?

Use long polling when you need **near-real-time updates** but want to stay on **plain HTTP** (easier through corporate proxies and firewalls, simpler infrastructure than maintaining WebSocket servers everywhere). WebSockets are better for **bidirectional**, **low-latency**, **high-frequency** streams (chat, games, collaborative cursors). Long polling is a reasonable compromise for **notification-style** or **status** updates where the server mostly **pushes** state to the client without needing a full duplex channel.

## Motivation

This project was built for learning:

- How **short** and **long** polling work in practice.
- Where **long polling** fits compared with **WebSockets**.
- How async work (here, a simulated EC2 deployment) can be reflected in a database and observed via HTTP.

## Features

- **Gin HTTP API** on port **9000**.
- **POST `/servers`** – Triggers a background job that simulates server provisioning (`createEC2`): updates `p_servers.status` through `TODO` → `IN PROGRESS` → `DONE` with delays.
- **GET `/short/status/:server_id`** – **Short polling**: one read of current status from MySQL.
- **GET `/long/status/:server_id?status=...`** – **Long polling**: blocks until the row’s `status` equals the `status` query parameter (internal poll loop with sleep), then returns JSON.
- **MySQL** persistence for status (table `p_servers`).
- Optional **Docker Compose** for local MySQL (add a `docker-compose.yml` next to this app or reuse one from the repo; point the DSN at the exposed port, e.g. `3307`).

## Tech stack

- **Language:** [Go](https://go.dev/) (1.21+; `go.mod` may specify a newer toolchain)
- **HTTP framework:** [Gin](https://github.com/gin-gonic/gin)
- **Database:** [MySQL 8](https://www.mysql.com/) via [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
- **Containers:** [Docker Compose](https://docs.docker.com/compose/) (optional)

## Prerequisites

- [Go](https://go.dev/dl/) installed
- MySQL reachable with a database and table matching the app (see below)
- (Optional) Docker and Docker Compose for the bundled MySQL service

## Installation

1. **Clone or open the project** and enter the `polling` directory:

   ```bash
   cd polling
   ```

2. **Install Go modules**

   ```bash
   go mod download
   ```

   Or:

   ```bash
   go mod tidy
   ```

3. **Configure the database connection**

   In `main.go`, the DSN is set in `init()` (e.g. `user:password@tcp(localhost:3307)/prototypes`). Adjust **user**, **password**, **host**, **port**, and **database** to match your MySQL instance.

4. **Create schema** (example)

   Ensure a table such as:

   ```sql
   CREATE TABLE IF NOT EXISTS p_servers (
     server_id INT PRIMARY KEY,
     status VARCHAR(64) NOT NULL
   );
   INSERT INTO p_servers (server_id, status) VALUES (1, 'IDLE')
     ON DUPLICATE KEY UPDATE status = VALUES(status);
   ```

   The sample code updates `server_id = 1` in `createEC2`.

5. **Optional: MySQL with Docker Compose**

   From the `polling` directory:

   ```bash
   docker compose up -d
   ```

   Create a MySQL user/password that matches your DSN (Compose in this repo sets `MYSQL_ROOT_PASSWORD`; you may need a dedicated user `user` with password `password`, or point the DSN at `root`).

## Usage

### Start the server

```bash
go run .
```

The API listens on **`:9000`**.

### Trigger a simulated deployment

```bash
curl -X POST http://localhost:9000/servers
```

Response example:

```json
{ "submitted": "ok" }
```

A goroutine runs `createEC2(1)`, which updates status over time in the database.

### Short polling (single snapshot)

```bash
curl "http://localhost:9000/short/status/1"
```

Response example:

```json
{ "status": "IN PROGRESS" }
```

Call repeatedly from the client to simulate **short polling** (timer-based refresh).

### Long polling (wait until status matches)

```bash
curl "http://localhost:9000/long/status/1?status=DONE"
```

This request **blocks** until `p_servers.status` for `server_id` `1` equals `DONE` (or matches whatever value you pass), then returns:

```json
{ "status": "DONE" }
```

Use this to illustrate **long polling**: one HTTP request that completes when the backend state catches up, instead of many rapid short polls.

## Notes

- This is a **learning prototype**, not production-ready: hardcoded DSN, minimal error handling, and a simplified long-poll loop.
- Align **Docker Compose** credentials and **MySQL users** with the DSN in `main.go`, or move secrets to environment variables for real deployments.
