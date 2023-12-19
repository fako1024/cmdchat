package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// CORS returns a CORS middleware
func CORS() echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{
			echo.HeaderAuthorization,
			echo.HeaderContentLength,
			echo.HeaderContentType,
		},
		AllowCredentials: true,
		AllowMethods:     []string{echo.GET},
	})
}
