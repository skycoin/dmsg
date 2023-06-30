// package main cmd/dmsgclientproxy/dmsgclientproxy.go
package main

import (
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	r.Use(reverseProxyMiddleware("http://localhost:8080"))

	r.Run(":80") //nolint
}

func reverseProxyMiddleware(targetURL string) gin.HandlerFunc {
	target, err := url.Parse(targetURL)
	if err != nil {
		panic(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
