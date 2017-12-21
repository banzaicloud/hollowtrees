package conf

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

func Init() {

	viper.AddConfigPath("$HOME/conf")
	viper.AddConfigPath("./")
	viper.AddConfigPath("./conf")

	viper.SetConfigName("config")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	fmt.Printf("Using config: %s\n", viper.ConfigFileUsed())
	viper.SetEnvPrefix("hollowtrees")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("dev.aws.region", "eu-west-1")
	viper.SetDefault("dev.engine.bufferSize", "100")
	viper.SetDefault("dev.plugin.address", "localhost:8888")
	viper.SetDefault("dev.engine.intervalInSeconds", "3")
	viper.SetDefault("dev.engine.reevaluateIntervalInSeconds", "60")
}
