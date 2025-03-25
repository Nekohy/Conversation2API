package main

import (
	"Conversation2API/processMessage"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"github.com/tidwall/gjson"
	"io"
	"strings"
)

type CaptureWriter struct {
	gin.ResponseWriter
	body         []byte
	streamBuffer bytes.Buffer // 使用bytes.Buffer管理流式数据
	messages     strings.Builder
}

// NonStreamMiddleware 提取非流式的情况
func (w *CaptureWriter) NonStreamMiddleware() {
	result := gjson.GetBytes(w.body, "choices.0.choices.message.content")
	if result.Exists() {
		w.messages.WriteString(result.String())
	}
}

// StreamMiddleware 提取流式数据的事件
func (w *CaptureWriter) StreamMiddleware() {
	data := w.streamBuffer.Bytes()
	events := bytes.Split(data, []byte("\n\n"))

	// 处理所有完整的事件（最后一个可能不完整，需保留）
	for i := 0; i < len(events)-1; i++ {
		event := events[i]
		if len(event) == 0 {
			continue
		}

		// 内联的 JSON 数据提取逻辑
		var jsonData []byte
		lines := bytes.Split(event, []byte("\n"))
		for _, line := range lines {
			line = bytes.TrimSpace(line)
			if bytes.HasPrefix(line, []byte("data: ")) {
				data := bytes.TrimPrefix(line, []byte("data: "))
				jsonData = append(jsonData, data...)
			}
		}

		if len(jsonData) == 0 {
			continue
		}

		// 原始数据处理逻辑
		if result := gjson.GetBytes(jsonData, "choices.0.delta.content"); result.Exists() {
			fmt.Printf("Stream Content: %s\n", result.String())
			w.messages.WriteString(result.String())
		}
	}

	// 更新缓冲区，保留未处理的事件部分
	if len(events) > 0 {
		w.streamBuffer.Reset()
		w.streamBuffer.Write(events[len(events)-1])
	} else {
		w.streamBuffer.Reset()
	}
}

// Write 覆写 gin.ResponseWriter 的 Write 方法
func (w *CaptureWriter) Write(data []byte) (int, error) {
	if contentType := w.Header().Get("Content-Type"); strings.HasPrefix(contentType, "text/event-stream") {
		// 写入流式缓冲区并处理事件
		w.streamBuffer.Write(data)
		w.StreamMiddleware()
	} else if strings.HasPrefix(contentType, "application/json") {
		w.body = append(w.body, data...)
		w.NonStreamMiddleware()
	}
	return w.ResponseWriter.Write(data)
}

func CaptureMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 替换为自定义Writer
		cw := &CaptureWriter{
			ResponseWriter: c.Writer,
			body:           make([]byte, 0),
			streamBuffer:   bytes.Buffer{},
			messages:       strings.Builder{},
		}

		// 定义Message结构体
		var messages []processMessage.Message
		// 检查 Content-Type 是否为 application/json
		if contentType := c.Request.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
			c.AbortWithStatusJSON(400, gin.H{"error": "Invalid Content-Type"})
			return
		}

		// 读取请求体内容
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(500, gin.H{"error": "Failed to read request body"})
			return
		}
		// 恢复请求体以便后续使用
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		bodyStr := string(bodyBytes)

		// 验证 JSON 有效性
		if !gjson.Valid(bodyStr) {
			c.AbortWithStatusJSON(400, gin.H{"error": "Invalid JSON"})
			return
		}
		// 解构并加入到 messages 字段
		messagesResult := gjson.Get(bodyStr, "messages")
		if !messagesResult.Exists() || !messagesResult.IsArray() {
			c.AbortWithStatusJSON(400, gin.H{"error": "messages field required"})
			return
		}
		if err := json.Unmarshal([]byte(messagesResult.Raw), &messages); err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": "Invalid messages format"})
			return
		}

		c.Writer = cw
		ctx := c.Request.Context()
		done := make(chan struct{})
		// 响应体处理完成后关闭连接并把消息写入流式响应（非正常关闭就直接就当作失败了）
		go func() {
			select {
			case <-ctx.Done():
				err := ctx.Err()
				fmt.Printf("意外断开: %s %s, 错误: %v\n", c.Request.Method, c.Request.URL.Path, err)
				cw.messages.Reset()
				cw.messages.WriteString(fmt.Sprintf("我不认为有人会有相同的回答，不过我的朋友，你的连接大抵真的是炸掉了-%s", uuid.NewV4().String()))
			case <-done:
				fmt.Printf("请求正常处理完成: %s %s\n", c.Request.Method, c.Request.URL.Path)
			}
			messages = append(messages, processMessage.Message{
				Role:    "assistant",
				Content: cw.messages.String(),
			})
			cw.messages.Reset()
			// 数据库操作
		}()

		defer close(done) // 防止goroutine泄漏
		c.Next()
	}
}

func main() {
	g := gin.Default()
	g.Use(CaptureMiddleware())
	g.GET("/normal", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "standard response"})
	})
}
