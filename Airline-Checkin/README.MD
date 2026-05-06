# Airline Check-in System

This prototype compares three seat-booking implementations under concurrency to demonstrate SQL locking behavior.

## Approaches

- `approach1`: no row lock on seat selection
- `approach2`: `FOR UPDATE`
- `approach3`: `FOR UPDATE SKIP LOCKED`

Details and observations for each are in:
- `approach1/explain.md`
- `approach2/explain.md`
- `approach3/explain.md`

## Prerequisites

- Go 1.22+
- Docker (for MySQL)

## Start MySQL

```bash
docker run --name mysql-server -e MYSQL_ROOT_PASSWORD=password -p 3307:3306 -v mysql-data:/var/lib/mysql -d mysql:latest
```

## Create DB and app user

```bash
docker exec -it mysql-server mysql -u root -ppassword
```

```sql
CREATE DATABASE IF NOT EXISTS prototypes;
CREATE USER IF NOT EXISTS 'relog'@'%' IDENTIFIED BY 'relog';
GRANT ALL PRIVILEGES ON *.* TO 'relog'@'%';
FLUSH PRIVILEGES;
```

## Seed initial data

From the `prototypes` repo root:

```bash
docker exec -i mysql-server mysql -u root -ppassword < Airline-Checkin/seed.sql
```

Seed contents:
- `trips`: 1 row -> `SOUTHWEST-101`
- `users`: 120 rows with generated names and numeric suffixes (examples: `Zoila Rau 001`, `Andrew Miller 002`)
- `seats`: 120 rows named `1-A` to `20-F`, all with `trip_id = 1` and `user_id = NULL`

## Run each approach

From `Airline-Checkin`:

```bash
MYSQL_USER=root MYSQL_PASSWORD=password MYSQL_PORT=3307 MYSQL_DB=prototypes go run ./approach1/main.go
MYSQL_USER=root MYSQL_PASSWORD=password MYSQL_PORT=3307 MYSQL_DB=prototypes go run ./approach2/main.go
MYSQL_USER=root MYSQL_PASSWORD=password MYSQL_PORT=3307 MYSQL_DB=prototypes go run ./approach3/main.go
```

Each run prints one summary line like:

```text
RESULT approach=2 users=120 success=120 no_seat=0 failures=0 assigned=120 duration_ms=200
```

## Benchmark comparison

Latest benchmark snapshots are in `benchmark/results.txt`.

Current highlights:
- `approach1` has race contention and poor allocation (very low assigned seat count).
- `approach2` and `approach3` both assign all 120 seats correctly.
- `approach3` is significantly faster than `approach2` because `SKIP LOCKED` avoids lock wait chains.
