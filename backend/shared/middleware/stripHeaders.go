package middleware

import (
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
)

var internalResponseHeaders = []string{
	utils.HeaderCorrelationID,
	utils.HeaderCallerService,
	utils.HeaderOriginService,
	utils.HeaderInternalToken,
	utils.HeaderInternalService,
	utils.HeaderServiceName,
}

// StripInternalHeaders keeps the internal headers off every response, whoever
// set them. Headers can't be changed once the status line is written, so the
// strip happens at that moment, not after the handlers return.
func StripInternalHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer = &stripWriter{ResponseWriter: c.Writer}
		c.Next()
	}
}

type stripWriter struct{ gin.ResponseWriter }

func (w *stripWriter) strip() {
	if w.Written() {
		return
	}
	for _, h := range internalResponseHeaders {
		w.Header().Del(h)
	}
}

func (w *stripWriter) WriteHeader(code int) {
	w.strip()
	w.ResponseWriter.WriteHeader(code)
}

func (w *stripWriter) WriteHeaderNow() {
	w.strip()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *stripWriter) Write(b []byte) (int, error) {
	w.strip()
	return w.ResponseWriter.Write(b)
}

func (w *stripWriter) WriteString(s string) (int, error) {
	w.strip()
	return w.ResponseWriter.WriteString(s)
}
