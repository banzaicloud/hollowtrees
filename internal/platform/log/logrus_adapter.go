// Copyright © 2018 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"os"

	"github.com/sirupsen/logrus"
)

type logrusAdapter struct {
	*logrus.Entry
}

// WithField adds a single field to the Entry
func (a *logrusAdapter) WithField(key string, value interface{}) Logger {
	return &logrusAdapter{a.Entry.WithField(key, value)}
}

// WithFields returns a new logger based on the original logger with
// the additional supplied fields.
func (a *logrusAdapter) WithFields(fields Fields) Logger {
	return &logrusAdapter{a.Entry.WithFields(logrus.Fields(fields))}
}

func NewLogrusLogger(config Config) Logger {
	logger := logrus.New()

	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors:             config.NoColor,
		EnvironmentOverrideColors: true,
	})

	switch config.Format {
	case "logfmt":
		// Already the default

	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	if level, err := logrus.ParseLevel(config.Level); err == nil {
		logger.SetLevel(level)
	}

	return &logrusAdapter{
		logrus.NewEntry(logger),
	}
}
