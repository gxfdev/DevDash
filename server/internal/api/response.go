package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type PaginatedResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Total   int64       `json:"total"`
	Page    int         `json:"page"`
	PerPage int         `json:"per_page"`
	Error   string      `json:"error,omitempty"`
}

const (
	CodeSuccess       = 0
	CodeInvalidParams = 40001
	CodeUnauthorized  = 40101
	CodeForbidden     = 40301
	CodeNotFound      = 40401
	CodeInternalError = 50001
	CodeRateLimited   = 42901
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: message,
		Data:    data,
	})
}

func SuccessPaginated(c *gin.Context, data interface{}, total int64, page, perPage int) {
	c.JSON(http.StatusOK, PaginatedResponse{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

func ErrorBadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    CodeInvalidParams,
		Message: "bad request",
		Error:   message,
	})
}

func ErrorUnauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    CodeUnauthorized,
		Message: "unauthorized",
		Error:   message,
	})
}

func ErrorForbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, Response{
		Code:    CodeForbidden,
		Message: "forbidden",
		Error:   message,
	})
}

func ErrorNotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, Response{
		Code:    CodeNotFound,
		Message: "not found",
		Error:   message,
	})
}

func ErrorInternal(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    CodeInternalError,
		Message: "internal server error",
		Error:   message,
	})
}

func ErrorRateLimited(c *gin.Context, message string) {
	c.JSON(http.StatusTooManyRequests, Response{
		Code:    CodeRateLimited,
		Message: "rate limited",
		Error:   message,
	})
}
