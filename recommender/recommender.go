package recommender

import (
	"encoding/json"
	"time"

	"sort"

	"strings"

	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/pricing"
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/sirupsen/logrus"
)

type AZRecommendation map[string][]InstanceTypeInfo

type InstanceTypeInfo struct {
	InstanceTypeName   string
	CurrentPrice       string
	AvgPriceFor24Hours float32
	OnDemandPrice      string
	SuggestedBidPrice  string
	CostScore          float32
	StabilityScore     float32
}

var log *logrus.Logger
var sess *session.Session

func init() {
	//TODO: region should come from config
	var err error
	sess, err = session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	})
	if err != nil {
		//TODO: handle error
	}
}

type ByNumericValue []string

func (a ByNumericValue) Len() int      { return len(a) }
func (a ByNumericValue) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByNumericValue) Less(i, j int) bool {
	intVal, _ := strconv.Atoi(strings.Split(a[i], " ")[0])
	intVal2, _ := strconv.Atoi(strings.Split(a[j], " ")[0])
	return intVal < intVal2
}

func RecommendSpotInstanceTypes(region string, az string, baseInstanceType string) (AZRecommendation, error) {

	log = conf.Logger()
	log.Info("received recommendation request: region/az/baseInstanceType: ", region, "/", az, "/", baseInstanceType)

	// validate region and base instance type

	pricingSvc := pricing.New(sess, aws.NewConfig())
	products, err := pricingSvc.GetProducts(&pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: []*pricing.Filter{
			{
				Type:  aws.String("TERM_MATCH"),
				Field: aws.String("operatingSystem"),
				Value: aws.String("Linux"),
			},
			{
				Type:  aws.String("TERM_MATCH"),
				Field: aws.String("instanceType"),
				Value: &baseInstanceType,
				//Value: aws.String("m4.xlarge"),
			},
			{
				Type:  aws.String("TERM_MATCH"),
				Field: aws.String("location"),
				Value: aws.String("EU (Ireland)"),
			},
			{
				Type:  aws.String("TERM_MATCH"),
				Field: aws.String("tenancy"),
				Value: aws.String("shared"),
			},
		},
	})
	if err != nil {
		// TODO: handle error
		log.Info("itt a hiba")
		log.Info(err.Error())
	}

	log.Info("len:", len(products.PriceList))

	// GET vcpu / memory of
	vcpu := products.PriceList[0]["product"].(map[string]interface{})["attributes"].(map[string]interface{})["vcpu"]
	memory := products.PriceList[0]["product"].(map[string]interface{})["attributes"].(map[string]interface{})["memory"]

	log.Info("vcpu:", vcpu, "memory:", memory)

	// GET products of the same memory / vcpu / operating system etc...

	vcpuValues, err := pricingSvc.GetAttributeValues(&pricing.GetAttributeValuesInput{
		ServiceCode:   aws.String("AmazonEC2"),
		AttributeName: aws.String("vcpu"),
	})
	if err != nil {
		//TODO: handle error
	}

	var vcpuStringValues []string
	for _, attrValue := range vcpuValues.AttributeValues {
		vcpuStringValues = append(vcpuStringValues, *attrValue.Value)
	}
	sort.Sort(ByNumericValue(vcpuStringValues))

	memValues, err := pricingSvc.GetAttributeValues(&pricing.GetAttributeValuesInput{
		ServiceCode:   aws.String("AmazonEC2"),
		AttributeName: aws.String("memory"),
	})
	if err != nil {
		//TODO: handle error
	}

	var memStringValues []string
	for _, attrValue := range memValues.AttributeValues {
		memStringValues = append(memStringValues, *attrValue.Value)
	}
	sort.Sort(ByNumericValue(memStringValues))

	log.Info("vcpustringValues: ", vcpuStringValues)

	instanceTypes := getProducts(memory.(string), vcpu.(string))
	memoryNext := getNextValue(memStringValues, memory.(string))
	vcpuNext := getNextValue(vcpuStringValues, vcpu.(string))
	if memoryNext != "" {
		largerMemInstances := getProducts(memoryNext, vcpu.(string))
		log.Info("largerMem ", largerMemInstances)
		for k, v := range largerMemInstances {
			instanceTypes[k] = v
		}
	}
	if vcpuNext != "" {
		largerCpuInstances := getProducts(memory.(string), vcpuNext)
		log.Info("largerCpu ", largerCpuInstances)
		for k, v := range largerCpuInstances {
			instanceTypes[k] = v
		}
	}
	if memoryNext != "" && vcpuNext != "" {
		largerInstances := getProducts(memoryNext, vcpuNext)
		log.Info("larger ", largerInstances)
		for k, v := range largerInstances {
			instanceTypes[k] = v
		}
	}

	log.Info(instanceTypes)

	instanceTypeStrings := make([]*string, 0, len(instanceTypes))
	for k := range instanceTypes {
		log.Info(k)
		log.Info(aws.String(k))
		instanceTypeStrings = append(instanceTypeStrings, aws.String(k))
	}

	log.Info(instanceTypeStrings)

	ec2Svc := ec2.New(sess, aws.NewConfig())
	history, err := ec2Svc.DescribeSpotPriceHistory(&ec2.DescribeSpotPriceHistoryInput{
		AvailabilityZone:    aws.String("us-east-1a"),
		StartTime:           aws.Time(time.Now()),
		ProductDescriptions: []*string{aws.String("Linux/UNIX")},
		InstanceTypes:       instanceTypeStrings,
	})
	if err != nil {
		// TODO: handle error
		log.Info(err.Error())
	}

	var instanceTypeInfo []InstanceTypeInfo

	log.Info(len(history.SpotPriceHistory))
	spots := make(map[string]string)
	for _, spot := range history.SpotPriceHistory {
		log.Info(*spot.InstanceType, ":", *spot.SpotPrice, " - ", *spot.AvailabilityZone, " - ", *spot.ProductDescription, " - ", *spot.Timestamp)
		spots[*spot.InstanceType] = *spot.SpotPrice
		instanceTypeInfo = append(instanceTypeInfo, InstanceTypeInfo{
			InstanceTypeName:   *spot.InstanceType,
			CurrentPrice:       *spot.SpotPrice,
			AvgPriceFor24Hours: 0.1,
			OnDemandPrice:      instanceTypes[*spot.InstanceType],
			SuggestedBidPrice:  instanceTypes[*spot.InstanceType],
			CostScore:          0.3,
			StabilityScore:     0.5,
		})
	}

	// get instance types based on base instance type from pricing api (based on cpus, mem, etc..)
	// compute cost/ondemand/stabilityscore/avgprice/currentprice in seleced AZ

	var azRecommendations = AZRecommendation{
		"eu-west-1a": instanceTypeInfo,
		"eu-west-1b": instanceTypeInfo,
	}

	return azRecommendations, nil
}

func getProducts(memory string, vcpu string) map[string]string {
	pricingSvc := pricing.New(sess, aws.NewConfig())
	products2, err := pricingSvc.GetProducts(&pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: []*pricing.Filter{
			{
				Type:  aws.String("TERM_MATCH"),
				Field: aws.String("operatingSystem"),
				Value: aws.String("Linux"),
			},
			{
				Type:  aws.String("TERM_MATCH"),
				Field: aws.String("memory"),
				Value: aws.String(memory),
			},
			{
				Type:  aws.String("TERM_MATCH"),
				Field: aws.String("vcpu"),
				Value: aws.String(vcpu),
			},
			{
				Type:  aws.String("TERM_MATCH"),
				Field: aws.String("location"),
				Value: aws.String("EU (Ireland)"),
			},
			{
				Type:  aws.String("TERM_MATCH"),
				Field: aws.String("tenancy"),
				Value: aws.String("shared"),
			},
		},
	})
	if err != nil {
		//TODO: error handling
	}

	instanceTypes := make(map[string]string)

	for _, price := range products2.PriceList {
		pjson, err := json.Marshal(price["product"].(map[string]interface{})["attributes"])
		if err != nil {
			// TODO: handle error
			log.Info(err.Error())
		}
		//log.Info(price["product"].(map[string]interface{})["attributes"].(map[string]interface{}))
		log.Info(string(pjson))

		instanceType := price["product"].(map[string]interface{})["attributes"].(map[string]interface{})["instanceType"]
		instanceTypes[instanceType.(string)] = "0.22"
		log.Info(instanceType.(string))

		onDemandTerm := price["terms"].(map[string]interface{})["OnDemand"].(map[string]interface{})
		for _, term := range onDemandTerm {
			priceDimensions := term.(map[string]interface{})["priceDimensions"].(map[string]interface{})
			for _, dimension := range priceDimensions {
				instanceTypes[instanceType.(string)] = dimension.(map[string]interface{})["pricePerUnit"].(map[string]interface{})["USD"].(string)
			}
		}

		p2json, err := json.Marshal(price["terms"].(map[string]interface{})["OnDemand"])
		if err != nil {
			// TODO: handle error
			log.Info(err.Error())
		}
		log.Info(string(p2json))
	}
	return instanceTypes
}

func getNextValue(values []string, value string) string {
	for i, val := range values {
		if val == value && i+1 < len(values) {
			return values[i+1]
		}
	}
	return ""
}
