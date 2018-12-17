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

package main

import (
	"github.com/goph/emperror"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/banzaicloud/hollowtrees/internal/platform/config"
)

var configuration config.Config

func configure() {
	config.Configure(viper.GetViper(), pflag.CommandLine)
	pflag.Parse()

	err := viper.ReadInConfig()
	if _, ok := err.(viper.ConfigFileNotFoundError); err != nil && !ok {
		panic(emperror.Wrap(err, "failed to read configuration"))
	}

	err = viper.Unmarshal(&configuration)
	if err != nil {
		panic(emperror.Wrap(err, "failed to unmarshal configuration"))
	}

	err = configuration.Validate()
	if err != nil {
		panic(emperror.Wrap(err, "cloud not validate configuration"))
	}
}
