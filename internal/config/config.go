// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft UG.  All rights reserved.5
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package config

import (
	"fmt"
	"reflect"
	"strconv"
)

type Config struct {
	NoPrompt       bool   `json:"no_prompt"        yaml:"no_prompt"        env:"KRAFTKIT_NO_PROMPT"    default:"false"`
	Editor         string `json:"editor"           yaml:"editor"           env:"KRAFTKIT_EDITOR"`
	Browser        string `json:"browser"          yaml:"browser"          env:"KRAFTKIT_BROWSER"`
	GitProtocol    string `json:"git_protocol"     yaml:"git_protocol"     env:"KRAFTKIT_GIT_PROTOCOL" default:"https"`
	Pager          string `json:"pager"            yaml:"pager"            env:"KRAFTKIT_PAGER"`
	HTTPUnixSocket string `json:"http_unix_socket" yaml:"http_unix_socket" env:"KRAFTKIT_HTTP_UNIX_SOCKET"`

	Paths struct {
		Plugins string `json:"plugins" yaml:"plugins,omitempty" env:"KRAFTKIT_PATHS_PLUGINS"`
		Config string  `json:"config"  yaml:"config,omitempty"  env:"KRAFTKIT_PATHS_CONFIG"`
	} `json:"paths" yaml:"paths,omitempty"`

	Log struct {
		Level      string `json:"level"      yaml:"level"      env:"KRAFTKIT_LOG_LEVEL"      default:"info"`
		Timestamps bool   `json:"timestamps" yaml:"timestamps" env:"KRAFTKIT_LOG_TIMESTAMPS" default:"false"`
		Type       string `json:"type"       yaml:"type"       env:"KRAFTKIT_LOG_TYPE"       default:"fancy"`
	} `json:"log" yaml:"log"`
}

type ConfigDetail struct {
	Key           string
	Description   string
	AllowedValues []string
}

// Descriptions of each configuration parameter as well as valid values
var configDetails = []ConfigDetail{
	{
		Key:         "no_prompt",
		Description: "toggle interactive prompting in the terminal",
	},
	{
		Key:         "editor",
		Description: "the text editor program to use for authoring text",
	},
	{
		Key:         "browser",
		Description: "the web browser to use for opening URLs",
	},
	{
		Key:         "git_protocol",
		Description: "the protocol to use for git clone and push operations",
		AllowedValues: []string{
			"https",
			"ssh",
		},
	},
	{
		Key:         "pager",
		Description: "the terminal pager program to send standard output to",
	},
	{
		Key:         "log.level",
		Description: "Set the logging verbosity",
		AllowedValues: []string{
			"fatal",
			"error",
			"warn",
			"info",
			"debug",
			"trace",
		},
	},
	{
		Key:         "log.type",
		Description: "Set the logging verbosity",
		AllowedValues: []string{
			"quiet",
			"basic",
			"fancy",
			"json",
		},
	},
	{
		Key:         "log.timestamps",
		Description: "Show timestamps with log output",
	},
}

func ConfigDetails() []ConfigDetail {
	return configDetails
}

func NewDefaultConfig() (*Config, error) {
	c := &Config{}

	if err := setDefaults(c); err != nil {
		return nil, fmt.Errorf("could not set defaults for config: %s", err)
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
			// Use the `default:""` tag as a hint for the value to set
			if err := setDefaultValue(
				v.Field(i).Addr(),
				v.Type().Field(i).Tag.Get("default"),
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

func findConfigDefault(needle, offset, def string, v reflect.Value) (string, string, string, reflect.Value, error) {
	if v.Kind() != reflect.Ptr {
		return needle, offset, def, v, fmt.Errorf("not a pointer value")
	}

	if needle == offset {
		return needle, offset, def, v, nil
	}

	v = reflect.Indirect(v)
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			name := v.Type().Field(i).Tag.Get("json")
			if len(name) == 0 {
				continue
			}

			check := name
			if len(offset) > 0 {
				check = offset + "." + name
			}

			dNeedle, dOffset, dDef, dv, dErr := findConfigDefault(
				needle,
				check,
				v.Type().Field(i).Tag.Get("default"),
				v.Field(i).Addr(),
			)

			if dOffset == needle {
				return dNeedle, dOffset, dDef, dv, dErr
			}
		}
	}

	return needle, offset, def, v, fmt.Errorf("could not find default for: %s", needle)
}
