package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func calculateHash(data string) string {
	hasher := sha256.New()
	hasher.Write([]byte(data))
	hash := hex.EncodeToString(hasher.Sum(nil))
	return fmt.Sprintf("W/\"%s\"", hash)
}

func ETagMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		writer := &responseWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = writer
		// let request happen
		c.Next()

		rawBytes := writer.body.Bytes()
		contentType := c.Writer.Header().Get("Content-Type")

		if len(rawBytes) > 0 && strings.Contains(contentType, "application/json") {
			hash := calculateHash(string(rawBytes))
			c.Writer.Header().Set("etag", hash)

			if c.Request.Method == http.MethodGet && c.Writer.Status() == http.StatusOK &&
				c.GetHeader("If-None-Match") == hash {
				c.Writer.WriteHeader(http.StatusNotModified)
				c.Writer.WriteHeaderNow()
				return
			}
		}

		writer.ResponseWriter.Write(rawBytes)
	}
}
