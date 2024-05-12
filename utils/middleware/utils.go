package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

func GetTransID(c *gin.Context) string {
	transID := c.GetHeader("transid")
	if transID == "" {
		transID = uuid.New().String()
		c.Header("transid", transID)
	}
	return transID
}

var Validate = validator.New()
