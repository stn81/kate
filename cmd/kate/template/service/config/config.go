package config

import (
	"fmt"

	"gopkg.in/ini.v1"
)

// Config defines the config interface
type Config interface {
	// SectionName return the section name
	SectionName() string

	// Load the config in the section specified in `SectionName()`
	Load(*ini.Section) error
}

// Load load all configs
func Load(file string) error {
	var (
		iniFile *ini.File
		err     error
	)

	if iniFile, err = ini.Load(file); err != nil {
		return fmt.Errorf("load config: %v", err)
	}

	configs := []Config{
		Main,
		Profiling,
		//kate:begin mysql
		DB,
		//kate:end mysql
		//kate:begin redis
		Redis,
		//kate:end redis
		HTTP,
		//kate:begin grpc
		GRPC,
		//kate:end grpc
	}

	for _, config := range configs {
		section := iniFile.Section(config.SectionName())
		if err = config.Load(section); err != nil {
			return fmt.Errorf("load config: section=%v, error=%v", config.SectionName(), err)
		}
	}
	return nil
}
