package api

import (
	"fmt"
	"net/http"

	"github.com/banzaicloud/hollowtrees/engine"
	"github.com/banzaicloud/hollowtrees/engine/aws"
	"github.com/banzaicloud/hollowtrees/recommender"
	"github.com/gin-gonic/gin"
	"gopkg.in/go-playground/validator.v8"
)

func ConfigureRoutes(router *gin.Engine) {
	v1 := router.Group("/api/v1/")
	{
		v1.GET("/recommender/:region", recommendSpotInstanceTypes)
		v1.POST("/hollowgroups", createHollowGroup)
	}
}

func recommendSpotInstanceTypes(c *gin.Context) {
	region := c.Param("region")
	baseInstanceType := c.DefaultQuery("baseInstanceType", "m4.xlarge")
	az := c.DefaultQuery("az", "")
	if response, err := recommender.RecommendSpotInstanceTypes(region, az, baseInstanceType); err != nil {
		// TODO: handle different error types
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": fmt.Sprintf("%s", err)})
	} else {
		c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "data": response})
	}
}

func createHollowGroup(c *gin.Context) {
	hgRequest := new(engine.HollowGroupRequest)
	if err := c.BindJSON(hgRequest); err != nil {
		if ve, ok := err.(validator.ValidationErrors); !ok {
			// TODO: not a validation error
			fmt.Println(err)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Missing required field", "error": ve.Error()})
		}
		return
	}
	awsEngine, err := aws.New("eu-west-1")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": fmt.Sprintf("%s", err)})
	}
	if response, err := awsEngine.CreateHollowGroup(hgRequest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": fmt.Sprintf("%s", err)})
	} else {
		c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "data": response})
	}
}
