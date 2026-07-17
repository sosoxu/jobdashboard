package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope is the unified API response.
type Envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Code: 0, Msg: "ok", Data: data})
}

func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, Envelope{Code: -1, Msg: msg})
}

func failBadRequest(c *gin.Context, msg string) {
	fail(c, http.StatusBadRequest, msg)
}

func failInternal(c *gin.Context, msg string) {
	fail(c, http.StatusInternalServerError, msg)
}
