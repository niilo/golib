package handlers

import (
	"net"
	"net/http"
)

func GetOriginalSourceIP(req *http.Request) string {
	if req.Header.Get("HTTP_CLIENT_IP") != "" {
		return req.Header.Get("HTTP_CLIENT_IP")
	}
	if req.Header.Get("X-ORIGINAL-SOURCE-IP") != "" {
		return req.Header.Get("X-ORIGINAL-SOURCE-IP")
	}
	if req.Header.Get("HTTP_X_FORWARDED_FOR") != "" {
		return req.Header.Get("HTTP_X_FORWARDED_FOR")
	}
	if req.Header.Get("HTTP_X_FORWARDED") != "" {
		return req.Header.Get("HTTP_X_FORWARDED")
	}
	if req.Header.Get("HTTP_X_CLUSTER_CLIENT_IP") != "" {
		return req.Header.Get("HTTP_X_CLUSTER_CLIENT_IP")
	}
	if req.Header.Get("X-REAL-IP") != "" {
		return req.Header.Get("X-REAL-IP")
	}
	if req.Header.Get("HTTP_FORWARDED_FOR") != "" {
		return req.Header.Get("HTTP_FORWARDED_FOR")
	}
	if req.Header.Get("HTTP_FORWARDED") != "" {
		return req.Header.Get("HTTP_FORWARDED")
	}

	addr, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		return addr
	}
	return req.RemoteAddr
}

func GetRemoteUser(req *http.Request) string {
	user := "-"
	if req.URL.User != nil && req.URL.User.Username() != "" {
		user = req.URL.User.Username()
	} else if len(req.Header["Remote-User"]) > 0 {
		user = req.Header["Remote-User"][0]
	}
	return user
}
