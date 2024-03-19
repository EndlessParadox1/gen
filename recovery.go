package gen

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
)

func trace(message string) string {
	var pcs [32]uintptr
	n := runtime.Callers(3, pcs[:])
	var s strings.Builder
	s.WriteString(message + "\nTraceback:\n")
	for _, pc := range pcs[:n] {
		fn := runtime.FuncForPC(pc)
		file, line := fn.FileLine(pc)
		s.WriteString(fmt.Sprintf("\t%s: %d\n", file, line))
	}
	return s.String()
}

func Recovery() HandlerFunc {
	return func(c *Context) {
		defer func() {
			if err := recover(); err != nil {
				message := err.(string)
				log.Printf("%s\n", trace(message))
				c.String(http.StatusInternalServerError, "Internal Server Error")
			}
		}()
		c.Next()
	}
}
