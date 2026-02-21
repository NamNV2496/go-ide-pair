package api

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewServer() {
	route := gin.Default()

	// allowedOrigins := getAllowedOrigins()
	route.Use(cors.New(cors.Config{
		// AllowOrigins:  allowedOrigins,
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:   []string{"Content-Length"},
		MaxAge:          12 * time.Hour,
	}))

	route.POST("/submit", submitHandler)
	route.Run(":8080")
}
