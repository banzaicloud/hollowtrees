package conf

import (
	"fmt"
	"log"
	"strings"

	"github.com/banzaicloud/hollowtrees/engine/types"
	"github.com/spf13/viper"
)

type PluginConfig struct {
	Name       string            `mapstructure:"name"`
	Address    string            `mapstructure:"address"`
	Type       string            `mapstructure:"type"`
	Properties map[string]string `mapstructure:"properties"`
}

type PluginConfigs []PluginConfig

func (p PluginConfigs) String() string {
	var result string
	for _, plugin := range p {
		result += fmt.Sprintf("\n - %s (%s)", plugin.Name, plugin.Address)
	}
	return result
}

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

	viper.SetDefault("global.bufferSize", 100)
	viper.SetDefault("global.bindAddr", ":9091")
}

func ReadPlugins() PluginConfigs {
	var plugins PluginConfigs
	err := viper.UnmarshalKey("action_plugins", &plugins)
	if err != nil {
		log.Fatalf("couldn't parse plugins config, %v", err)
	}
	// TODO: validate plugin type/properties
	return plugins
}

func ReadRules() types.Rules {
	var rules types.Rules
	err := viper.UnmarshalKey("rules", &rules)
	if err != nil {
		log.Fatalf("couldn't parse rules config, %v", err)
	}
	return rules
}
