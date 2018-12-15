package healthcheck

import "errors"

type Config struct {
	ListenAddress string
	Endpoint      string
}

// Validate checks that the configuration is valid.
func (c Config) Validate() error {
	if c.ListenAddress == "" {
		return errors.New("listen address must not be empty")
	}

	if c.Endpoint == "" {
		return errors.New("endpoint must not be empty")
	}

	return nil
}
