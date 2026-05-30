package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	dynamodb "dynamodb-on-mysql"
	"dynamodb-on-mysql/internal/db"
	"dynamodb-on-mysql/internal/types"
)

const demoTable = "demo_products"

func main() {
	ctx := context.Background()

	database, err := db.Open()
	if err != nil {
		log.Fatalf("connect to MySQL: %v\nHint: set DB_* env vars (see .env)", err)
	}
	defer database.Close()

	client := dynamodb.NewClient(database)
	if err := client.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	if err := ensureDemoTable(ctx, client); err != nil {
		log.Fatal(err)
	}

	if err := seedDemoData(ctx, client); err != nil {
		log.Fatal(err)
	}

	if err := runDemos(ctx, client); err != nil {
		log.Fatal(err)
	}
}

func ensureDemoTable(ctx context.Context, client *dynamodb.DynamoDBClient) error {
	_, err := client.DescribeTable(ctx, demoTable)
	if err == nil {
		return nil
	}
	if !errors.Is(err, types.ErrTableNotFound) {
		return err
	}

	return client.CreateTable(ctx, types.CreateTableParams{
		TableName: demoTable,
		PKName:    "product_id",
		SKName:    "variant",
		LSIs: []types.IndexDef{
			{IndexName: "by_category", SKName: "category"},
		},
		GSIs: []types.IndexDef{
			{IndexName: "by_brand", PKName: "brand", SKName: "price"},
		},
	})
}

func seedDemoData(ctx context.Context, client *dynamodb.DynamoDBClient) error {
	items := []types.Item{
		{
			"product_id": "p1",
			"variant":    "v1",
			"name":       "Widget",
			"category":   "hardware",
			"brand":      "acme",
			"price":      "9.99",
		},
		{
			"product_id": "p1",
			"variant":    "v2",
			"name":       "Widget Pro",
			"category":   "hardware",
			"brand":      "acme",
			"price":      "19.99",
		},
		{
			"product_id": "p2",
			"variant":    "v1",
			"name":       "Gadget",
			"brand":      "other",
			"price":      "4.50",
		},
	}

	for _, item := range items {
		if err := client.PutItem(ctx, demoTable, item); err != nil {
			return err
		}
	}
	return nil
}

func runDemos(ctx context.Context, client *dynamodb.DynamoDBClient) error {
	fmt.Println("=== DynamoDB on MySQL — demo ===")
	fmt.Println()

	item, err := client.GetItem(ctx, demoTable, types.Key{PK: "p1", SK: "v1"})
	if err != nil {
		return fmt.Errorf("GetItem: %w", err)
	}
	printResult("GetItem (p1, v1)", item)

	queryItems, err := client.Query(ctx, types.QueryParams{
		TableName: demoTable,
		PKValue:   "p1",
		SKOp:      types.OpGTE,
		SKValue:   "v1",
	})
	if err != nil {
		return fmt.Errorf("Query: %w", err)
	}
	printResult("Query partition p1 (sk >= v1)", queryItems)

	lsiItems, err := client.Query(ctx, types.QueryParams{
		TableName: demoTable,
		IndexName: "by_category",
		PKValue:   "p1",
		SKOp:      types.OpEQ,
		SKValue:   "hardware",
	})
	if err != nil {
		return fmt.Errorf("LSI Query: %w", err)
	}
	printResult("Query LSI by_category (p1, hardware)", lsiItems)

	gsiItems, err := client.Query(ctx, types.QueryParams{
		TableName: demoTable,
		IndexName: "by_brand",
		PKValue:   "acme",
	})
	if err != nil {
		return fmt.Errorf("GSI Query: %w", err)
	}
	printResult("Query GSI by_brand (acme)", gsiItems)

	scanItems, err := client.Scan(ctx, types.ScanParams{
		TableName: demoTable,
		Limit:     10,
	})
	if err != nil {
		return fmt.Errorf("Scan: %w", err)
	}
	printResult("Scan (limit 10)", scanItems)

	fmt.Println("Done. Inspect MySQL with:")
	fmt.Printf("  USE %s; SELECT * FROM %s;\n", envOr("DB_NAME", "dynamodb_emulator"), demoTable)
	return nil
}

func printResult(label string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Printf("%s: %v", label, v)
		return
	}
	fmt.Printf("--- %s ---\n%s\n\n", label, b)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
