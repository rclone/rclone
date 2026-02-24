package server

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// TODO: This is a disgusting hack to match the output of the internal quark command.
// They should return JSON instead of HTML!
func (s *Server) handleQuarkCommand() gin.HandlerFunc {
	return func(c *gin.Context) {
		res, err := s.b.RunQuarkCommand(c.Param("command"), strings.Split(c.Query("strInput"), " ")...)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		var out string

		switch res := res.(type) {
		case string:
			out = res

		default:
			b, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				_ = c.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			out = string(b)
		}

		tmp, err := template.New("quarkCommand").Parse(`<html><body><div class="content">{{.Content}}</div></body></html>`)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if err := tmp.Execute(c.Writer, map[string]string{
			"Content": template.HTMLEscapeString(out),
		}); err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}
}
