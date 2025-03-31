package family

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
)

func notSupportedApi(http *gin.Context) {
	http.JSON(500, map[string]interface{}{
		"error": function.ErrorMessage(".proLicenseFileIsCorrect").Error(),
		"code":  500,
	})
	http.Abort()
	return
}
