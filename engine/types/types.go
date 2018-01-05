package types

import (
	"fmt"
	"time"
)

type Alert struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
}

type Plugin struct {
	Name    string `mapstructure:"name"`
	Address string `mapstructure:"address"`
}

type Plugins []Plugin

type Rule struct {
	Name        string            `mapstructure:"name"`
	Description string            `mapstructure:"description"`
	EventType   string            `mapstructure:"event_type"`
	Plugins     []string          `mapstructure:"action_plugins"`
	Match       map[string]string `mapstructure:"match"`
}

type Rules []Rule

func (p Plugins) String() string {
	var result string
	for _, plugin := range p {
		result += fmt.Sprintf("\n - %s (%s)", plugin.Name, plugin.Address)
	}
	return result
}

func (r Rules) String() string {
	var result string
	for _, rule := range r {
		result += fmt.Sprintf("\n- Name: %s", rule.Name)
		result += fmt.Sprintf("\n  Description: %s", rule.Description)
		result += fmt.Sprintf("\n  EventType: %s", rule.EventType)
		result += fmt.Sprintf("\n  Plugins:")
		for _, p := range rule.Plugins {
			result += fmt.Sprintf("\n  - %s", p)
		}
		if len(rule.Match) > 0 {
			result += fmt.Sprintf("\n  Match:")
			for k, v := range rule.Match {
				result += fmt.Sprintf("\n  - %s = %s", k, v)
			}
		}
	}
	return result
}
