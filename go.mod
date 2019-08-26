module github.com/banzaicloud/hollowtrees

go 1.12

require (
	github.com/asaskevich/EventBus v0.0.0-20180315140547-d46933a94f05
	github.com/banzaicloud/bank-vaults/pkg/sdk v0.1.3-0.20190826065836-26d654c87254
	github.com/cloudevents/sdk-go v0.0.0-20181211100118-3a3d34a7231e
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-gonic/gin v1.4.0
	github.com/go-playground/locales v0.12.1 // indirect
	github.com/go-playground/universal-translator v0.16.0 // indirect
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/golang/protobuf v1.3.1
	github.com/goph/emperror v0.14.0
	github.com/goph/logur v0.5.0
	github.com/leodido/go-urn v1.1.0 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.8.1
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cast v1.3.0
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/ugorji/go/codec v0.0.0-20181204163529-d75b2dcb6bc8 // indirect
	golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc
	golang.org/x/sys v0.0.0-20190616124812-15dcb6c0061f // indirect
	golang.org/x/text v0.3.2 // indirect
	google.golang.org/grpc v1.22.0
	gopkg.in/go-playground/validator.v8 v8.18.2
	gopkg.in/go-playground/validator.v9 v9.29.1
	gopkg.in/yaml.v2 v2.2.2
)

replace (
	github.com/ugorji/go => github.com/ugorji/go/codec v1.1.7
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20181127025237-2b1284ed4c93
	k8s.io/client-go => k8s.io/client-go v2.0.0-alpha.0.0.20181213151034-8d9ed539ba31+incompatible
)
