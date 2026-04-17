package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func init() {
	_db, err := sql.Open("mysql", "user:password@tcp(localhost:3307)/prototypes")
	if err != nil {
		panic(err)
	}
	db = _db
}

func createEC2(serverID int) {
	fmt.Println("Creating  Server");
	_, err := db.Exec("UPDATE p_servers SET status = 'TODO' WHERE server_id = ?;", serverID);
	if err != nil {
		panic(err)
	}

	time.Sleep(5 * time.Second);
	_, err = db.Exec("UPDATE p_servers SET status = 'IN PROGRESS' WHERE server_id = ?;", serverID);
	if err != nil {
		panic(err)
	}
	fmt.Println("Server creation in progress");

	time.Sleep(10 * time.Second);
	_, err = db.Exec("UPDATE p_servers SET status = 'DONE' WHERE server_id = ?;", serverID);
	if err != nil {
		panic(err)
	}
	fmt.Println("Server creation completed");
}

func main() {
	router := gin.Default()
	router.POST("/servers", func(c *gin.Context) {
		go createEC2(1)
		c.JSON(http.StatusOK, map[string]interface{}{"submitted": "ok"})
	})

	router.GET("/short/status/:server_id", func(c *gin.Context) {
		serverID := c.Param("server_id")

		var status string
		row := db.QueryRow("SELECT status FROM p_servers WHERE server_id = ?;", serverID)
		if row.Err() != nil {
			panic(row.Err())
		}
		row.Scan(&status)
		c.JSON(http.StatusOK, map[string]interface{}{"status": status})
	})

	router.GET("/long/status/:server_id", func(c *gin.Context) {
		serverID := c.Param("server_id")
		currentStatus := c.Query("status")

		var status string
		for {
			row := db.QueryRow("SELECT status FROM p_servers WHERE server_id = ?;", serverID)
			if row.Err() != nil {
				panic(row.Err())
			}
			row.Scan(&status)
			if status == currentStatus {
				break
			}
			time.Sleep(1 * time.Second)
		}

		c.JSON(http.StatusOK, map[string]interface{}{"status": status})
	})

	router.Run(":9000")
}