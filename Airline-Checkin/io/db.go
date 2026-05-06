package io

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

// Init establishes a shared DB handle.
func Init() error {
	if DB != nil {
		return nil
	}

	user := getEnv("MYSQL_USER", "root")
	pass := getEnv("MYSQL_PASSWORD", "password")
	host := getEnv("MYSQL_HOST", "127.0.0.1")
	port := getEnv("MYSQL_PORT", "3307")
	name := getEnv("MYSQL_DB", "prototypes")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, pass, host, port, name)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return err
	}

	DB = db
	return nil
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
