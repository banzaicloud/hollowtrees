package recommender

import (
	"sort"
	"strconv"
	"strings"
	"time"

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
	floatVal, _ := strconv.ParseFloat(strings.Split(a[i], " ")[0], 32)
	floatVal2, _ := strconv.ParseFloat(strings.Split(a[j], " ")[0], 32)
	return floatVal < floatVal2
}

func RecommendSpotInstanceTypes(region string, az string, baseInstanceType string) (AZRecommendation, error) {

	log = conf.Logger()
	log.Info("received recommendation request: region/az/baseInstanceType: ", region, "/", az, "/", baseInstanceType)

	// TODO: validate region and base instance type

	pricingSvc := pricing.New(sess, aws.NewConfig())

	// TODO: this can be cached, product info won't change much
	vcpu, memory, err := getBaseProductInfo(pricingSvc, baseInstanceType)
	if err != nil {
		//TODO: handle error
	}

	// TODO: this can be cached, available memory/vcpu attributes won't change
	vcpuStringValues, err := getNumericSortedAttributeValues(pricingSvc, "vcpu")
	if err != nil {
		// TODO: handle error
		log.Info(err.Error())
	}

	memStringValues, err := getNumericSortedAttributeValues(pricingSvc, "memory")
	if err != nil {
		// TODO: handle error
		log.Info(err.Error())
	}

	instanceTypes, err := getSimilarInstanceTypesWithPriceInfo(pricingSvc, memory, vcpu, memStringValues, vcpuStringValues)
	if err != nil {
		// TODO: handle error
		log.Info(err.Error())
	}

	instanceTypeInfo, err := getSpotPriceInfo(instanceTypes)
	if err != nil {
		// TODO: handle error
		log.Info(err.Error())
	}

	var azRecommendations = AZRecommendation{
		"eu-west-1a": instanceTypeInfo,
		"eu-west-1b": instanceTypeInfo,
	}

	return azRecommendations, nil
}

func getBaseProductInfo(pricingSvc *pricing.Pricing, baseInstanceType string) (string, string, error) {
	log.Info("getting product info (memory,vcpu) of base instance type: ", baseInstanceType)
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
		log.Info(err.Error())
		return "", "", err
	}
	vcpu := products.PriceList[0]["product"].(map[string]interface{})["attributes"].(map[string]interface{})["vcpu"].(string)
	memory := products.PriceList[0]["product"].(map[string]interface{})["attributes"].(map[string]interface{})["memory"].(string)
	log.Info("Product info of base instance type: ", "vcpu: ", vcpu, " memory: ", memory)
	return vcpu, memory, nil
}

func getNumericSortedAttributeValues(pricingSvc *pricing.Pricing, attribute string) ([]string, error) {
	log.Info("Getting available ", attribute, " values from AWS API.")
	attrValues, err := pricingSvc.GetAttributeValues(&pricing.GetAttributeValuesInput{
		ServiceCode:   aws.String("AmazonEC2"),
		AttributeName: aws.String(attribute),
	})
	if err != nil {
		return nil, err
	}
	var stringValues []string
	for _, attrValue := range attrValues.AttributeValues {
		stringValues = append(stringValues, *attrValue.Value)
	}
	sort.Sort(ByNumericValue(stringValues))
	log.Info(attribute, " attribute values sorted: ", stringValues)
	return stringValues, nil
}

func getSimilarInstanceTypesWithPriceInfo(pricingSvc *pricing.Pricing, memory string, vcpu string, memStringValues []string, vcpuStringValues []string) (map[string]string, error) {
	log.Info("Getting instance types with memory/vcpu profile similar to: ", memory, "/", vcpu)
	instanceTypes, err := getProductsWithMemAndVcpu(pricingSvc, memory, vcpu)
	if err != nil {
		// TODO: handle error
		return nil, err
	}
	memoryNext := getNextValue(memStringValues, memory)
	vcpuNext := getNextValue(vcpuStringValues, vcpu)
	if memoryNext != "" {
		largerMemInstances, err := getProductsWithMemAndVcpu(pricingSvc, memoryNext, vcpu)
		if err != nil {
			// TODO: handle error
			return nil, err
		}
		log.Info("largerMem ", largerMemInstances)
		for k, v := range largerMemInstances {
			instanceTypes[k] = v
		}
	}
	if vcpuNext != "" {
		largerCpuInstances, err := getProductsWithMemAndVcpu(pricingSvc, memory, vcpuNext)
		if err != nil {
			// TODO: handle error
			return nil, err
		}
		log.Info("largerCpu ", largerCpuInstances)
		for k, v := range largerCpuInstances {
			instanceTypes[k] = v
		}
	}
	if memoryNext != "" && vcpuNext != "" {
		largerInstances, err := getProductsWithMemAndVcpu(pricingSvc, memoryNext, vcpuNext)
		if err != nil {
			// TODO: handle error
			return nil, err
		}
		log.Info("larger ", largerInstances)
		for k, v := range largerInstances {
			instanceTypes[k] = v
		}
	}
	log.Info("Instance types found with similar profiles: ", instanceTypes)
	return instanceTypes, nil
}

func getProductsWithMemAndVcpu(pricingSvc *pricing.Pricing, memory string, vcpu string) (map[string]string, error) {
	log.Info("Getting instance types and on demand prices with specification: [memory: ", memory, ", vcpu: ", vcpu, "]")
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
		return nil, err
	}

	instanceTypes := make(map[string]string)

	for _, price := range products.PriceList {
		// TODO: check if these values are present so we won't get values from the map with invalid keys
		instanceType := price["product"].(map[string]interface{})["attributes"].(map[string]interface{})["instanceType"]
		onDemandTerm := price["terms"].(map[string]interface{})["OnDemand"].(map[string]interface{})
		for _, term := range onDemandTerm {
			priceDimensions := term.(map[string]interface{})["priceDimensions"].(map[string]interface{})
			for _, dimension := range priceDimensions {
				instanceTypes[instanceType.(string)] = dimension.(map[string]interface{})["pricePerUnit"].(map[string]interface{})["USD"].(string)
			}
		}
	}
	log.Info("instance types and on demand prices [memory: ", memory, ", vcpu: ", vcpu, "]: ", instanceTypes)
	return instanceTypes, nil
}

func getNextValue(values []string, value string) string {
	for i, val := range values {
		if val == value && i+1 < len(values) {
			return values[i+1]
		}
	}
	return ""
}

func getSpotPriceInfo(instanceTypes map[string]string) ([]InstanceTypeInfo, error) {
	instanceTypeStrings := make([]*string, 0, len(instanceTypes))
	for k := range instanceTypes {
		instanceTypeStrings = append(instanceTypeStrings, aws.String(k))
	}
	log.Info("Getting current spot price of these instance types: ", instanceTypeStrings)
	ec2Svc := ec2.New(sess, aws.NewConfig())
	history, err := ec2Svc.DescribeSpotPriceHistory(&ec2.DescribeSpotPriceHistoryInput{
		AvailabilityZone:    aws.String("us-east-1a"),
		StartTime:           aws.Time(time.Now()),
		ProductDescriptions: []*string{aws.String("Linux/UNIX")},
		InstanceTypes:       instanceTypeStrings,
	})
	if err != nil {
		// TODO: handle error
		return nil, err
	}
	var instanceTypeInfo []InstanceTypeInfo
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
	log.Info("Instance type info found: ", instanceTypeInfo)
	return instanceTypeInfo, nil
}
