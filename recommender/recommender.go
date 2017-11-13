package recommender

import (
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
)

type AZRecommendation struct {
	AzName       string
	InstaceTypes []InstanceTypeInfo
}

type InstanceTypeInfo struct {
	InstanceTypeName   string
	CurrentPrice       float32
	AvgPriceFor24Hours float32
	OnDemandPrice      float32
	CostScore          float32
	StabilityScore     float32
}

var log *logrus.Logger

func RecommendSpotInstanceTypes(c *gin.Context) {
	log = conf.Logger()

	region := c.Param("region")
	log.Info(region)

	baseInstanceType := c.DefaultQuery("baseInstanceType", "m4.xlarge")
	log.Info(baseInstanceType)

	az := c.DefaultQuery("az", "")
	log.Info(az)

	// validate region and base instance type
	// get instance types based on base instance type from pricing api (based on cpus, mem, etc..)
	// compute cost/ondemand/stabilityscore/avgprice/currentprice in seleced AZ

	var response = []AZRecommendation{
		AZRecommendation{
			AzName: "eu-west-1a",
			InstaceTypes: []InstanceTypeInfo{
				InstanceTypeInfo{
					InstanceTypeName:   "m4.xlarge",
					CurrentPrice:       0.2,
					AvgPriceFor24Hours: 0.1,
					OnDemandPrice:      0.22,
					CostScore:          0.3,
					StabilityScore:     0.5,
				},
				InstanceTypeInfo{
					InstanceTypeName:   "c5.xlarge",
					CurrentPrice:       0.065,
					AvgPriceFor24Hours: 0.07,
					OnDemandPrice:      0.25,
					CostScore:          0.95,
					StabilityScore:     0.99,
				},
			},
		},
		AZRecommendation{
			AzName: "eu-west-1b",
			InstaceTypes: []InstanceTypeInfo{
				InstanceTypeInfo{
					InstanceTypeName:   "m4.xlarge",
					CurrentPrice:       0.1,
					AvgPriceFor24Hours: 0.08,
					OnDemandPrice:      0.22,
					CostScore:          0.6,
					StabilityScore:     0.8,
				},
				InstanceTypeInfo{
					InstanceTypeName:   "c5.xlarge",
					CurrentPrice:       0.06,
					AvgPriceFor24Hours: 0.065,
					OnDemandPrice:      0.25,
					CostScore:          0.99,
					StabilityScore:     0.99,
				},
			},
		},
	}

	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "data": response})
}
