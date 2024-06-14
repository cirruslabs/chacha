package fail

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func Fail(c echo.Context, status int, format string, args ...interface{}) error {
	message := fmt.Sprintf(format, args...)

	zap.L().Warn(message)

	jsonResp := struct {
		Message string `json:"message"`
	}{
		Message: message,
	}

	return c.JSON(status, &jsonResp)
}
