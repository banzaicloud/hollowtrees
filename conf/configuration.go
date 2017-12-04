package conf

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

func init() {

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
}
