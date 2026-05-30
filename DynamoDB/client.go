package dynamodb

import (
	"context"
	"database/sql"

	"dynamodb-on-mysql/internal/catalog"
	"dynamodb-on-mysql/internal/index"
	itemmgr "dynamodb-on-mysql/internal/item"
	"dynamodb-on-mysql/internal/query"
	"dynamodb-on-mysql/internal/table"
	"dynamodb-on-mysql/internal/types"
)

type DynamoDBClient struct {
	db *sql.DB
}

func NewClient(db *sql.DB) *DynamoDBClient {
	return &DynamoDBClient{db: db}
}

func (c *DynamoDBClient) Bootstrap(ctx context.Context) error {
	return catalog.Bootstrap(c.db)
}

func (c *DynamoDBClient) CreateTable(ctx context.Context, params types.CreateTableParams) error {
	return table.CreateTable(ctx, c.db, params)
}

func (c *DynamoDBClient) DeleteTable(ctx context.Context, tableName string) error {
	return table.DeleteTable(ctx, c.db, tableName)
}

func (c *DynamoDBClient) DescribeTable(ctx context.Context, tableName string) (catalog.TableMeta, error) {
	return table.DescribeTable(ctx, c.db, tableName)
}

func (c *DynamoDBClient) PutItem(ctx context.Context, tableName string, item types.Item) error {
	return itemmgr.PutItem(ctx, c.db, tableName, item)
}

func (c *DynamoDBClient) GetItem(ctx context.Context, tableName string, key types.Key) (types.Item, error) {
	return itemmgr.GetItem(ctx, c.db, tableName, key)
}

func (c *DynamoDBClient) DeleteItem(ctx context.Context, tableName string, key types.Key) error {
	return itemmgr.DeleteItem(ctx, c.db, tableName, key)
}

func (c *DynamoDBClient) UpdateItem(ctx context.Context, tableName string, key types.Key, update types.UpdateParams) error {
	return itemmgr.UpdateItem(ctx, c.db, tableName, key, update)
}

func (c *DynamoDBClient) Query(ctx context.Context, params types.QueryParams) ([]types.Item, error) {
	return query.Query(ctx, c.db, params)
}

func (c *DynamoDBClient) Scan(ctx context.Context, params types.ScanParams) ([]types.Item, error) {
	return query.Scan(ctx, c.db, params)
}

func (c *DynamoDBClient) CreateGSI(ctx context.Context, tableName string, def types.IndexDef) error {
	return index.CreateGSI(ctx, c.db, tableName, def)
}

func (c *DynamoDBClient) DB() *sql.DB {
	return c.db
}
