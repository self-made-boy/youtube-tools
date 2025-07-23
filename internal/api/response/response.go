package response

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// Response 表示 API 响应
type Response struct {
	Code    string      `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 返回成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: SUCCESS,
		Data: data,
	})
}

// SuccessWithMessage 返回带消息的成功响应
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    SUCCESS,
		Message: message,
		Data:    data,
	})
}

// Fail 返回失败响应
func Fail(c *gin.Context, httpStatus int, code string, err error) {
	message := GetMessage(code)
	if err != nil {
		message += ": " + err.Error()
	}

	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// FailWithMessage 返回带自定义消息的失败响应
func FailWithMessage(c *gin.Context, httpStatus int, code string, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// BadRequest 返回客户端错误响应
func BadRequest(c *gin.Context, code string, err error) {
	Fail(c, http.StatusBadRequest, code, err)
}

// NotFound 返回资源未找到响应
func NotFound(c *gin.Context, code string, err error) {
	Fail(c, http.StatusNotFound, code, err)
}

// ServerError 返回服务器错误响应
func ServerError(c *gin.Context, err error) {
	Fail(c, http.StatusInternalServerError, SERVER_ERROR, err)
}