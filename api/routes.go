package api

import (
	"github.com/gin-gonic/gin"
	"github.com/banzaicloud/hollowtrees/recommender"
	"net/http"
)

func ConfigureRoutes(router *gin.Engine) {
	v1 := router.Group("/api/v1/")
	{
		v1.GET("/recommender/:region", recommendSpotInstanceTypes)
	}
}

func recommendSpotInstanceTypes(c *gin.Context) {
	region := c.Param("region")
	baseInstanceType := c.DefaultQuery("baseInstanceType", "m4.xlarge")
	az := c.DefaultQuery("az", "")
	response := recommender.RecommendSpotInstanceTypes(region, baseInstanceType, az)
	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "data": response})
}
