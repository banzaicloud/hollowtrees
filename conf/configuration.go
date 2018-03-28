package conf

import (
	"fmt"
	"log"
	"strings"

	"github.com/banzaicloud/hollowtrees/engine/types"
	"github.com/patrickmn/go-cache"
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
	viper.SetDefault("global.defaultActionFlowConcurrency", 10)
	viper.SetDefault("global.bindAddr", ":9091")
}

func ReadPlugins() PluginConfigs {
	var plugins PluginConfigs
	err := viper.UnmarshalKey("action_plugins", &plugins)
	if err != nil {
		log.Fatalf("couldn't parse plugins config, %v", err)
	}
	for _, p := range plugins {
		if p.Type == "" {
			log.Fatalf("couldn't parse plugins config, plugin type is required [grpc/fn]: %s", p.Name)
		}
	}
	// TODO: validate plugin type/properties
	return plugins
}

func ReadActionFlows() types.ActionFlows {
	var afs types.ActionFlows
	err := viper.UnmarshalKey("action_flows", &afs)
	if err != nil {
		log.Fatalf("couldn't parse action flows config, %v", err)
	}
	for i := range afs {
		if afs[i].ConcurrentFlows == 0 {
			afs[i].ConcurrentFlows = viper.GetInt("global.defaultActionFlowConcurrency")
		}
		if afs[i].RepeatCooldown <= 0 {
			afs[i].RepeatCooldown = cache.NoExpiration
		}
	}
	return afs
}
