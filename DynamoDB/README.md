# DynamoDB on MySQL

A Go prototype that implements a DynamoDB-like key-value store on top of a single-node MySQL database. Each DynamoDB table maps to one or more MySQL tables, with Local Secondary Indexes (LSI) and Global Secondary Indexes (GSI) kept in sync inside MySQL transactions at the application layer.

## Motivation

This project was created for **learning purposes** — to understand how DynamoDB concepts (partition keys, sort keys, indexes, and strongly consistent reads) can be modeled and implemented using familiar relational database primitives.

## Features

- **Table management** — create, describe, and delete tables with partition key and optional sort key
- **Item operations** — `PutItem`, `GetItem`, `DeleteItem`, and `UpdateItem` (SET / REMOVE)
- **Query & Scan** — partition-key queries with sort-key conditions (`=`, `<`, `<=`, `>`, `>=`, `BETWEEN`, `begins_with`) and paginated table scans
- **Local Secondary Indexes (LSI)** — same partition key, alternate sort key; sparse index support
- **Global Secondary Indexes (GSI)** — alternate partition/sort keys; add after table creation with automatic backfill
- **Transactional index sync** — base table and shadow index tables updated in the same transaction on every write
- **System catalog** — metadata stored in `_ddb_tables` and `_ddb_indexes` with in-process caching
- **Strong consistency** — single-node MySQL; all reads are strongly consistent

## Tech Stack

| Category | Technology |
|----------|------------|
| Language | [Go](https://go.dev/) 1.24+ |
| Database | [MySQL](https://www.mysql.com/) 8.x (JSON column support required) |
| Driver | [`github.com/go-sql-driver/mysql`](https://github.com/go-sql-driver/mysql) |
| Storage model | JSON `item_data` column + extracted `pk` / `sk` columns for indexing |

## Project Structure

```
DynamoDB/
├── client.go                 # Public DynamoDBClient API
├── internal/
│   ├── catalog/              # System catalog bootstrap and metadata cache
│   ├── db/                   # MySQL connection pool
│   ├── errors/               # MySQL error mapping
│   ├── index/                # LSI/GSI sync and CreateGSI backfill
│   ├── item/                 # PutItem, GetItem, DeleteItem, UpdateItem
│   ├── query/                # Query and Scan
│   ├── table/                # CreateTable, DeleteTable
│   ├── types/                # Shared domain types
│   └── validate/             # Table and index name validation
├── sql/
│   └── bootstrap.sql         # Catalog DDL
└── tests/
    └── integration_test.go   # End-to-end tests against real MySQL
```

## Prerequisites

- **Go 1.24+**
- **MySQL 8.x** running locally (native install or Docker)
- A MySQL user with permission to create databases and tables

## Installation

### 1. Clone and enter the project

```bash
cd prototypes/DynamoDB
```

### 2. Install Go dependencies

```bash
go mod download
```

### 3. Set up MySQL

**Option A — existing local MySQL**

Log in and create the application database:

```sql
CREATE DATABASE IF NOT EXISTS dynamodb_emulator;
```

**Option B — Docker**

```bash
docker run -d --name ddb-mysql \
  -e MYSQL_ROOT_PASSWORD=your_password \
  -e MYSQL_DATABASE=dynamodb_emulator \
  -p 3306:3306 \
  mysql:8
```

### 4. Configure environment variables

Create or edit `.env` in the project root with your MySQL credentials:

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | MySQL host | `localhost` |
| `DB_PORT` | MySQL port | `3306` |
| `DB_USER` | MySQL user | `root` |
| `DB_PASSWORD` | MySQL password | *(required)* |
| `DB_NAME` | Database name | `dynamodb_emulator` |

Load the environment before running commands:

```bash
set -a && source .env && set +a
```

> **Note:** The application reads configuration from environment variables via `os.Getenv`. The `.env` file is not loaded automatically — you must export the variables in your shell.

### 5. Verify the build

```bash
go build ./...
```

## Usage

### Connect and bootstrap

```go
package main

import (
    "context"
    "log"

    dynamodb "dynamodb-on-mysql"
    "dynamodb-on-mysql/internal/db"
    "dynamodb-on-mysql/internal/types"
)

func main() {
    ctx := context.Background()

    database, err := db.Open()
    if err != nil {
        log.Fatal(err)
    }
    defer database.Close()

    client := dynamodb.NewClient(database)
    if err := client.Bootstrap(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### Create a table and write items

```go
// Partition key + sort key table with an LSI and GSI
err := client.CreateTable(ctx, types.CreateTableParams{
    TableName: "products",
    PKName:    "product_id",
    SKName:    "variant",
    LSIs: []types.IndexDef{
        {IndexName: "by_category", SKName: "category"},
    },
    GSIs: []types.IndexDef{
        {IndexName: "by_brand", PKName: "brand", SKName: "price"},
    },
})

item := types.Item{
    "product_id": "p1",
    "variant":    "v1",
    "category":   "books",
    "brand":      "acme",
    "price":      "9.99",
}
client.PutItem(ctx, "products", item)
```

### Read, update, and delete

```go
// Get by key
got, err := client.GetItem(ctx, "products", types.Key{PK: "p1", SK: "v1"})

// Update attributes
client.UpdateItem(ctx, "products", types.Key{PK: "p1", SK: "v1"}, types.UpdateParams{
    Set:    map[string]any{"price": "12.99"},
    Remove: []string{"deprecated_field"},
})

// Delete
client.DeleteItem(ctx, "products", types.Key{PK: "p1", SK: "v1"})
```

### Query and scan

```go
// Query base table with a sort-key range
items, err := client.Query(ctx, types.QueryParams{
    TableName: "products",
    PKValue:   "p1",
    SKOp:      types.OpBetween,
    SKValue:   "v1",
    SKValue2:  "v9",
})

// Query via GSI
items, err = client.Query(ctx, types.QueryParams{
    TableName: "products",
    IndexName: "by_brand",
    PKValue:   "acme",
})

// Scan with pagination
page, err := client.Scan(ctx, types.ScanParams{
    TableName:        "products",
    Limit:            25,
    ExclusiveStartPK: lastPK,
    ExclusiveStartSK: lastSK,
})
```

### Add a GSI to an existing table

```go
err := client.CreateGSI(ctx, "inventory", types.IndexDef{
    IndexName: "by_warehouse",
    PKName:    "warehouse",
})
// Existing items are backfilled automatically
```

## Running Tests

Integration tests require a running MySQL instance with valid credentials:

```bash
set -a && source .env && set +a
go test ./tests/... -v -count=1
```

Tests are skipped gracefully if MySQL is unavailable.

### Test Results

All integration tests pass against MySQL 8.x / 9.x with the configured local instance.

| Test | Status | What it verifies |
|------|--------|------------------|
| `TestCreateTableAndPutGet` | PASS | Creates a PK+SK table, writes an item, and reads it back with JSON fidelity |
| `TestUpdateAndDeleteItem` | PASS | Updates item attributes (SET/REMOVE) and confirms deletion removes the row |
| `TestQuerySortKeyConditions` | PASS | Queries a partition with a `BETWEEN` sort-key condition and returns the correct subset |
| `TestLSIAndGSI` | PASS | LSI/GSI shadow tables stay in sync; sparse items are excluded; index queries return expected results |
| `TestCreateGSIBackfill` | PASS | Adds a GSI to a populated table and backfills all existing items into the index |
| `TestCatalogCache` | PASS | Table metadata is stored in the catalog and retrievable via the in-process cache |

```
PASS
ok   dynamodb-on-mysql/tests   0.4s
```

## Design Notes

| DynamoDB Concept | MySQL Implementation |
|------------------|----------------------|
| Table | MySQL table with JSON `item_data` column |
| Partition / Sort Key | `pk` and `sk` VARCHAR columns extracted from item JSON |
| LSI | Shadow table `{table}__lsi__{index}` synced in-transaction |
| GSI | Shadow table `{table}__gsi__{index}` synced in-transaction |
| Strongly consistent read | Single-node MySQL — always consistent |

**Out of scope:** TTL, DynamoDB Streams, TransactWriteItems, and multi-node replication.

## License

This is a learning prototype. Use and modify freely for educational purposes.
