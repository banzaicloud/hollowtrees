package flows

import (
	"time"

	"github.com/goph/emperror"
	"github.com/pkg/errors"

	"github.com/banzaicloud/hollowtrees/internal/plugin"
)

// FlowConfig holds configuration values for an action flow
type FlowConfig struct {
	Name    string   `mapstructure:"name"`
	Plugins []string `mapstructure:"plugins"`

	Description   string            `mapstructure:"description"`
	AllowedEvents []string          `mapstructure:"allowedEvents"`
	GroupBy       []string          `mapstructure:"groupBy"`
	Filters       map[string]string `mapstructure:"filters"`
	Cooldown      time.Duration     `mapstructure:"cooldown"`
}

type FlowConfigs map[string]FlowConfig

// Validate validates flow configuration
func (c FlowConfig) Validate(plugins plugin.PluginManager, id string) error {
	if c.Name == "" {
		return errors.New("name must be set")
	}

	if len(c.Plugins) == 0 {
		return emperror.WrapWith(errors.New("no plugins defined"), "invalid flow config", "flow", id)
	}

	_, err := plugins.GetByNames(c.Plugins...)
	if err != nil {
		return emperror.WrapWith(err, "invalid flow", "flow", id)
	}

	return nil
}
