package gin

import (
	"log"
	"net/http"
	"strings"

	. "github.com/lflxp/rpcx-gateway"
	"github.com/devopsxp/gateway/middlewares"
	"github.com/gin-gonic/gin"
)

type Server struct {
	addr string
	g    *gin.Engine
	jwt  middlewares.JwtAuthorizator
}

// New returns a server.
func New(addr string, jwta middlewares.JwtAuthorizator) *Server {
	return &Server{
		addr: addr,
		jwt:  jwta,
	}
}

// NewWithGin returns a server with preconfigured gin.
func NewWithGin(addr string, g *gin.Engine, jwta middlewares.JwtAuthorizator) *Server {
	return &Server{
		addr: addr,
		g:    g,
		jwt:  jwta,
	}
}

// RegisterHandler configures the handler to handle http rpcx invoke.
// It wraps ServiceHandler into httprouter.Handle.
// Add JwtAuthorizator support
func (s *Server) RegisterHandler(base string, handler ServiceHandler) {
	g := s.g
	if g == nil {
		g = gin.Default()
	}
	h := wrapServiceHandler(handler)

	var jwtMiddleware = middlewares.NewGinJwtMiddlewares(s.jwt)
	apiGroup := g.Group(base)
	apiGroup.Use(jwtMiddleware.MiddlewareFunc())

	apiGroup.POST("", h)
	apiGroup.GET("", h)
	apiGroup.PUT("", h)
	s.g = g
}

func wrapServiceHandler(handler ServiceHandler) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		r := ctx.Request
		w := ctx.Writer
		if r.Header.Get(XServicePath) == "" {
			servicePath := ctx.Param("servicePath")
			if strings.HasPrefix(servicePath, "/") {
				servicePath = servicePath[1:]
			}

			r.Header.Set(XServicePath, servicePath)
		}

		servicePath := r.Header.Get(XServicePath)
		messageID := r.Header.Get(XMessageID)
		wh := w.Header()
		if messageID != "" {
			wh.Set(XMessageID, messageID)
		}

		authorization := r.Header.Get("Authorization")
		if authorization != "" {
			log.Println("au", authorization)
			wh.Set("Authorization", authorization)
		}

		meta, payload, err := handler(r, servicePath)
		for k, v := range meta {
			wh.Set(k, v)
		}

		if err == nil {
			ctx.Data(http.StatusOK, "application/octet-stream", payload)
			return
		}

		rh := r.Header
		for k, v := range rh {
			if strings.HasPrefix(k, "X-RPCX-") && len(v) > 0 {
				wh.Set(k, v[0])
			}
		}

		wh.Set(XMessageStatusType, "Error")
		wh.Set(XErrorMessage, err.Error())
		ctx.String(http.StatusOK, err.Error())
	}
}

func (s *Server) Serve() error {
	return s.g.Run(s.addr)
}
