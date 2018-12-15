package promalert

import "github.com/pkg/errors"

type Config struct {
	// HTTP listen address
	ListenAddress string
}

// Validate checks that the configuration is valid.
func (c Config) Validate() error {
	if c.ListenAddress == "" {
		return errors.New("listen address must not be empty")
	}

	return nil
}
