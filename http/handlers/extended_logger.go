package handlers

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

type ExtendedLogRecord struct {
	http.ResponseWriter

	ip                    string
	user                  string
	time                  time.Time
	method, uri, protocol string
	status                int
	contentLength         int64
	elapsedTime           time.Duration
	referer               string
	userAgent             string
}

func (r *ExtendedLogRecord) Write(p []byte) (int, error) {
	written, err := r.ResponseWriter.Write(p)
	r.contentLength += int64(written)
	return written, err
}

func (r *ExtendedLogRecord) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (h *ExtendedLogHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	logHandler := &ExtendedLogRecord{
		ResponseWriter: w,
		ip:             GetOriginalSourceIP(req),
		user:           GetRemoteUser(req),
		time:           start.UTC(),
		method:         req.Method,
		uri:            req.RequestURI,
		protocol:       req.Proto,
		status:         http.StatusOK,
		elapsedTime:    time.Duration(0),
		referer:        req.Referer(),
		userAgent:      req.UserAgent(),
	}

	h.handler.ServeHTTP(logHandler, req)
	logHandler.elapsedTime = time.Since(start) / time.Millisecond
	logHandler.Log(h.out)

}

func (r *ExtendedLogRecord) Log(out io.Writer) {
	fmt.Fprintf(out, "%s - %s [%s] \"%s %s %s\" %d %d %d \"%s\" \"%s\"\n",
		r.ip,
		r.user,
		r.time.Format("02/Jan/2006:15:04:05 -0700"),
		r.method,
		r.uri,
		r.protocol,
		r.status,
		r.contentLength,
		r.elapsedTime,
		r.referer,
		r.userAgent,
	)
}

type ExtendedLogHandler struct {
	handler http.Handler
	out     io.Writer
}

func NewExtendedLogHandler(handler http.Handler, out io.Writer) http.Handler {
	return &ExtendedLogHandler{
		handler: handler,
		out:     out,
	}
}
