// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
)

const (
	DefaultNoPrompt      = "false"
	DefaultNoParallel    = "true"
	DefaultNoEmojis      = "false"
	DefaultGitProtocol   = "https"
	DefaultLogLevel      = "info"
	DefaultLogTimestamps = "false"
	DefaultLogType       = "fancy"
	DefaultRuntimeDir    = "/var/kraftkit"
	DefaultEventsPidFile = "/var/kraftkit/events.pid"
	DefaultManifestIndex = "https://manifests.kraftkit.sh/index.yaml"
)

func Defaults() map[string]string {
	return map[string]string{
		"KRAFTKIT_NO_PROMPT":      DefaultNoPrompt,
		"KRAFTKIT_NO_PARALLEL":    DefaultNoParallel,
		"KRAFTKIT_NO_EMOJIS":      DefaultNoEmojis,
		"KRAFTKIT_GIT_PROTOCOL":   DefaultGitProtocol,
		"KRAFTKIT_LOG_LEVEL":      DefaultLogLevel,
		"KRAFTKIT_LOG_TIMESTAMPS": DefaultLogTimestamps,
		"KRAFTKIT_LOG_TYPE":       DefaultLogType,
		"KRAFTKIT_RUNTIME_DIR":    DefaultRuntimeDir,
		"KRAFTKIT_EVENTS_PIDFILE": DefaultEventsPidFile,
	}
}

func NewDefaultConfig() (*Config, error) {
	c := &Config{}

	if err := setDefaults(c); err != nil {
		return nil, fmt.Errorf("could not set defaults for config: %s", err)
	}

	// Add default path for plugins..
	if len(c.Paths.Plugins) == 0 {
		c.Paths.Plugins = filepath.Join(DataDir(), "plugins")
	}

	// ..for configuration files..
	if len(c.Paths.Config) == 0 {
		c.Paths.Config = filepath.Join(ConfigDir())
	}

	// ..for manifest files..
	if len(c.Paths.Manifests) == 0 {
		c.Paths.Manifests = filepath.Join(DataDir(), "manifests")
	}

	// ..and for cached source files
	if len(c.Paths.Sources) == 0 {
		c.Paths.Sources = filepath.Join(DataDir(), "sources")
	}

	if len(c.Unikraft.Manifests) == 0 {
		c.Unikraft.Manifests = append(c.Unikraft.Manifests, DefaultManifestIndex)
	}

	return c, nil
}

func setDefaults(s interface{}) error {
	return setDefaultValue(reflect.ValueOf(s), "")
}

func setDefaultValue(v reflect.Value, def string) error {
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("not a pointer value")
	}

	v = reflect.Indirect(v)

	switch v.Kind() {
	case reflect.Int:
		if len(def) > 0 {
			i, err := strconv.ParseInt(def, 10, 64)
			if err != nil {
				return fmt.Errorf("could not parse default integer value: %s", err)
			}
			v.SetInt(i)
		}

	case reflect.String:
		if len(def) > 0 {
			v.SetString(def)
		}

	case reflect.Bool:
		if len(def) > 0 {
			b, err := strconv.ParseBool(def)
			if err != nil {
				return fmt.Errorf("could not parse default boolean value: %s", err)
			}
			v.SetBool(b)
		} else {
			// Assume false by default
			v.SetBool(false)
		}

	case reflect.Struct:
		// Iterate over the struct fields
		for i := 0; i < v.NumField(); i++ {
			// Use the `env` tag to look up the default value
			def = Defaults()[v.Type().Field(i).Tag.Get("env")]
			if err := setDefaultValue(
				v.Field(i).Addr(),
				def,
			); err != nil {
				return err
			}
		}

	// TODO: Arrays? Maps?

	default:
		// Ignore this value and property entirely
		return nil
	}

	return nil
}
