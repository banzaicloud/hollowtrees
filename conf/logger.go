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

package conf

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var logger *logrus.Logger

func Logger() *logrus.Logger {
	if logger == nil {
		logger = logrus.New()
		switch viper.GetString("log.level") {
		case "debug":
			logrus.SetLevel(logrus.DebugLevel)
		case "info":
			logrus.SetLevel(logrus.InfoLevel)
		case "warn":
			logrus.SetLevel(logrus.WarnLevel)
		case "error":
			logrus.SetLevel(logrus.ErrorLevel)
		case "fatal":
			logrus.SetLevel(logrus.FatalLevel)
		default:
			logrus.WithField("log.level", viper.GetString("log.level")).Warning("Invalid log level. Defaulting to info.")
			logrus.SetLevel(logrus.InfoLevel)
		}

		switch viper.GetString("log.format") {
		case "text":
			logrus.SetFormatter(new(logrus.TextFormatter))
		case "json":
			logrus.SetFormatter(new(logrus.JSONFormatter))
		default:
			logrus.WithField("log.format", viper.GetString("log.format")).Warning("Invalid log format. Defaulting to text.")
			logrus.SetFormatter(new(logrus.TextFormatter))
		}

		logger.SetLevel(logrus.DebugLevel)
	}
	return logger
}
