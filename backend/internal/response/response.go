package response

import "github.com/gin-gonic/gin"

type Envelope struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"requestId,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(200, Envelope{Code: "SUCCESS", Message: "操作成功", Data: data, RequestID: requestID(c)})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(201, Envelope{Code: "SUCCESS", Message: "创建成功", Data: data, RequestID: requestID(c)})
}

func Error(c *gin.Context, status int, code, message string, details interface{}) {
	c.JSON(status, Envelope{Code: code, Message: message, Details: details, RequestID: requestID(c)})
}

func requestID(c *gin.Context) string {
	return c.GetString("requestId")
}
