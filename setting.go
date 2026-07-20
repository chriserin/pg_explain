package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type Setting struct {
	name    string
	setting string
}

func (setting *Setting) View() string {
	spaceAvailable := 32 - ansi.StringWidth(setting.name)
	return fmt.Sprintf("%s: %*s", setting.name, spaceAvailable, setting.setting)
}

var settingPositions []string = []string{
	"work_mem",
	"random_page_cost",
	"join_collapse_limit",
	"effective_cache_size",
	"max_parallel_workers_per_gather",
	"cluster_name",
}

var excludedFromNextSettings []string = []string{
	"cluster_name",
}

func (setting *Setting) FindPosition() int {
	for i, sPos := range settingPositions {
		if sPos == setting.name {
			return i
		}
	}
	return -1
}

func SettingCompare(a, b Setting) int {
	return a.FindPosition() - b.FindPosition()
}

func (setting Setting) Sql() string {
	return fmt.Sprintf("SET %s = '%s'", setting.name, setting.setting)
}

func (setting Setting) Marshal() string {
	return fmt.Sprintf("%s=%s", setting.name, setting.setting)
}

func SettingUnmarshal(settingstr string) Setting {
	splitstr := strings.Split(settingstr, "=")
	return Setting{name: splitstr[0], setting: splitstr[1]}
}

var SettingsValues map[string][]string = map[string][]string{
	"work_mem":                        []string{"4MB", "40MB", "400MB", "800MB", "1GB", "2GB", "3GB", "4GB"},
	"random_page_cost":                []string{"1", "1.1", "2", "3", "4"},
	"join_collapse_limit":             []string{"1", "2", "3", "4", "5", "6", "7", "8"},
	"effective_cache_size":            []string{"4GB"},
	"max_parallel_workers_per_gather": []string{"0", "1", "2", "3", "4", "5", "6", "7", "8"},
}

func (setting *Setting) IncrementSetting() {
	values := SettingsValues[setting.name]
	for i, value := range values {
		if value == setting.setting {
			if i+1 < len(values) {
				setting.setting = values[i+1]
			}
			break
		}
	}
}

func (setting *Setting) DecrementSetting() {
	values := SettingsValues[setting.name]
	for i, value := range values {
		if value == setting.setting {
			if i-1 >= 0 {
				setting.setting = values[i-1]
			}
			break
		}
	}
}
