package config

import (
	"fmt"
	"path"

	"github.com/stn81/kate/app"
)

// Main is the log config instance
var Main = &MainConfig{}

// MainConfig defines the Main config
type MainConfig struct {
	PidFile string
	LogDir  string
}

// SectionName implements the `Config.SectionName()` method
func (conf *MainConfig) SectionName() string {
	return "main"
}

// Load implements the `Config.Load()` method
func (conf *MainConfig) Load(section *ini.Section) error {
	defaultPidFile := path.Join(app.GetHomeDir(), "run", fmt.Sprintf("%s.pid", app.GetName()))
	conf.PidFile = section.Key("pid_file").MustString(defaultPidFile)
	conf.LogDir = section.Key("log_dir").MustString("")
	return nil
}
