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

type ActionFlow struct {
	Name            string            `mapstructure:"name"`
	Description     string            `mapstructure:"description"`
	EventType       string            `mapstructure:"event_type"`
	ConcurrentFlows int               `mapstructure:"concurrent_flows"`
	Cooldown        time.Duration     `mapstructure:"flow_cooldown"`
	RepeatCooldown  time.Duration     `mapstructure:"repeat_cooldown"`
	Retries         int               `mapstructure:"retries"`
	GroupBy         []string          `mapstructure:"group_by"`
	Plugins         []string          `mapstructure:"action_plugins"`
	Match           map[string]string `mapstructure:"match"`
}

type ActionFlows []*ActionFlow

func (a ActionFlows) String() string {
	var result string
	for _, af := range a {
		result += fmt.Sprintf("\n- Name: %s", af.Name)
		result += fmt.Sprintf("\n  Description: %s", af.Description)
		result += fmt.Sprintf("\n  Event Type: %s", af.EventType)
		result += fmt.Sprintf("\n  Concurrent Flows: %d", af.ConcurrentFlows)
		result += fmt.Sprintf("\n  Cooldown: %v", af.Cooldown)
		result += fmt.Sprintf("\n  Repeat Cooldown: %v", af.RepeatCooldown)
		result += fmt.Sprintf("\n  Retries: %v", af.Retries)
		result += fmt.Sprintf("\n  Group by: %v", af.GroupBy)
		result += fmt.Sprintf("\n  Plugins:")
		for _, p := range af.Plugins {
			result += fmt.Sprintf("\n  - %s", p)
		}
		if len(af.Match) > 0 {
			result += fmt.Sprintf("\n  Match:")
			for k, v := range af.Match {
				result += fmt.Sprintf("\n  - %s = %s", k, v)
			}
		}
	}
	return result
}
