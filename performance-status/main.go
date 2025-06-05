package main

import (
	"math/rand"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

func main() {
	server_port := os.Getenv("PORT")
	r := gin.Default()
	r.GET("/status", func(c *gin.Context) {
		c.String(200, strconv.Itoa(rand.Intn(100)))
	})
	r.Run(":" + server_port)
}
