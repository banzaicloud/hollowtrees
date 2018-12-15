package plugin

import (
	"github.com/goph/emperror"
	"github.com/pkg/errors"
)

// PluginConfig describes a plugin configuration
type PluginConfig struct {
	Name    string `mapstructure:"name"`
	Type    string `mapstructure:"type"`
	Address string `mapstructure:"address"`
}

type PluginConfigs []PluginConfig

// Validate validates plugin configuration
func (c PluginConfig) Validate() error {
	if c.Name == "" {
		return errors.New("name must be set")
	}

	if c.Type != "grpc" {
		return emperror.With(errors.New("invalid plugin type"), "type", c.Type)
	}

	if c.Type == "grpc" && c.Address == "" {
		return errors.New("address must not be empty for a GRPC plugin")
	}

	return nil
}
