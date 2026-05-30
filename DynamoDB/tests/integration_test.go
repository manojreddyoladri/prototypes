package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"dynamodb-on-mysql"
	"dynamodb-on-mysql/internal/catalog"
	"dynamodb-on-mysql/internal/db"
	"dynamodb-on-mysql/internal/types"
)

func testClient(t *testing.T) *dynamodb.DynamoDBClient {
	t.Helper()

	database, err := db.Open()
	if err != nil {
		t.Skipf("MySQL not available: %v", err)
	}

	ctx := context.Background()
	client := dynamodb.NewClient(database)
	if err := client.Bootstrap(ctx); err != nil {
		database.Close()
		t.Fatalf("bootstrap: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
	})

	return client
}

func uniqueTable(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func TestCreateTableAndPutGet(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	tableName := uniqueTable("users")

	err := client.CreateTable(ctx, types.CreateTableParams{
		TableName: tableName,
		PKName:    "user_id",
		SKName:    "sort_key",
	})
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	t.Cleanup(func() {
		_ = client.DeleteTable(ctx, tableName)
	})

	item := types.Item{
		"user_id":  "u1",
		"sort_key": "profile",
		"name":     "Alice",
		"age":      float64(30),
	}

	if err := client.PutItem(ctx, tableName, item); err != nil {
		t.Fatalf("PutItem: %v", err)
	}

	got, err := client.GetItem(ctx, tableName, types.Key{PK: "u1", SK: "profile"})
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if got["name"] != "Alice" {
		t.Fatalf("expected name Alice, got %v", got["name"])
	}
}

func TestUpdateAndDeleteItem(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	tableName := uniqueTable("orders")

	if err := client.CreateTable(ctx, types.CreateTableParams{
		TableName: tableName,
		PKName:    "order_id",
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	t.Cleanup(func() {
		_ = client.DeleteTable(ctx, tableName)
	})

	if err := client.PutItem(ctx, tableName, types.Item{
		"order_id": "o1",
		"status":   "pending",
	}); err != nil {
		t.Fatalf("PutItem: %v", err)
	}

	if err := client.UpdateItem(ctx, tableName, types.Key{PK: "o1"}, types.UpdateParams{
		Set:    map[string]any{"status": "shipped"},
		Remove: []string{"extra"},
	}); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}

	got, err := client.GetItem(ctx, tableName, types.Key{PK: "o1"})
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if got["status"] != "shipped" {
		t.Fatalf("expected shipped, got %v", got["status"])
	}

	if err := client.DeleteItem(ctx, tableName, types.Key{PK: "o1"}); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}

	got, err = client.GetItem(ctx, tableName, types.Key{PK: "o1"})
	if err != nil {
		t.Fatalf("GetItem after delete: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil item after delete")
	}
}

func TestQuerySortKeyConditions(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	tableName := uniqueTable("events")

	if err := client.CreateTable(ctx, types.CreateTableParams{
		TableName: tableName,
		PKName:    "pk",
		SKName:    "sk",
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	t.Cleanup(func() {
		_ = client.DeleteTable(ctx, tableName)
	})

	for _, sk := range []string{"a", "b", "c", "d"} {
		if err := client.PutItem(ctx, tableName, types.Item{"pk": "p1", "sk": sk}); err != nil {
			t.Fatalf("PutItem: %v", err)
		}
	}

	items, err := client.Query(ctx, types.QueryParams{
		TableName: tableName,
		PKValue:   "p1",
		SKOp:      types.OpBetween,
		SKValue:   "b",
		SKValue2:  "c",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestLSIAndGSI(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	tableName := uniqueTable("products")

	if err := client.CreateTable(ctx, types.CreateTableParams{
		TableName: tableName,
		PKName:    "product_id",
		SKName:    "variant",
		LSIs: []types.IndexDef{
			{IndexName: "by_category", SKName: "category"},
		},
		GSIs: []types.IndexDef{
			{IndexName: "by_brand", PKName: "brand", SKName: "price"},
		},
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	t.Cleanup(func() {
		_ = client.DeleteTable(ctx, tableName)
	})

	if err := client.PutItem(ctx, tableName, types.Item{
		"product_id": "p1",
		"variant":    "v1",
		"category":   "books",
		"brand":      "acme",
		"price":      "9.99",
	}); err != nil {
		t.Fatalf("PutItem: %v", err)
	}

	if err := client.PutItem(ctx, tableName, types.Item{
		"product_id": "p2",
		"variant":    "v1",
		"brand":      "other",
		"price":      "1.00",
	}); err != nil {
		t.Fatalf("PutItem sparse: %v", err)
	}

	lsiItems, err := client.Query(ctx, types.QueryParams{
		TableName: tableName,
		IndexName: "by_category",
		PKValue:   "p1",
		SKOp:      types.OpEQ,
		SKValue:   "books",
	})
	if err != nil {
		t.Fatalf("LSI query: %v", err)
	}
	if len(lsiItems) != 1 {
		t.Fatalf("expected 1 LSI item, got %d", len(lsiItems))
	}

	gsiItems, err := client.Query(ctx, types.QueryParams{
		TableName: tableName,
		IndexName: "by_brand",
		PKValue:   "acme",
	})
	if err != nil {
		t.Fatalf("GSI query: %v", err)
	}
	if len(gsiItems) != 1 {
		t.Fatalf("expected 1 GSI item, got %d", len(gsiItems))
	}
}

func TestCreateGSIBackfill(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	tableName := uniqueTable("inventory")

	if err := client.CreateTable(ctx, types.CreateTableParams{
		TableName: tableName,
		PKName:    "sku",
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	t.Cleanup(func() {
		_ = client.DeleteTable(ctx, tableName)
	})

	for _, sku := range []string{"s1", "s2", "s3"} {
		if err := client.PutItem(ctx, tableName, types.Item{
			"sku":      sku,
			"warehouse": "east",
		}); err != nil {
			t.Fatalf("PutItem: %v", err)
		}
	}

	if err := client.CreateGSI(ctx, tableName, types.IndexDef{
		IndexName: "by_warehouse",
		PKName:    "warehouse",
	}); err != nil {
		t.Fatalf("CreateGSI: %v", err)
	}

	items, err := client.Query(ctx, types.QueryParams{
		TableName: tableName,
		IndexName: "by_warehouse",
		PKValue:   "east",
	})
	if err != nil {
		t.Fatalf("GSI query after backfill: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 backfilled items, got %d", len(items))
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("DB_HOST") == "" {
		os.Setenv("DB_HOST", "localhost")
	}
	if os.Getenv("DB_NAME") == "" {
		os.Setenv("DB_NAME", "dynamodb_emulator")
	}
	os.Exit(m.Run())
}

func TestCatalogCache(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	tableName := uniqueTable("cache_test")

	if err := client.CreateTable(ctx, types.CreateTableParams{
		TableName: tableName,
		PKName:    "id",
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	t.Cleanup(func() {
		_ = client.DeleteTable(ctx, tableName)
	})

	meta, err := catalog.GetTable(client.DB(), tableName)
	if err != nil {
		t.Fatalf("GetTable: %v", err)
	}
	if meta.PKName != "id" {
		t.Fatalf("unexpected pk name: %s", meta.PKName)
	}
}
