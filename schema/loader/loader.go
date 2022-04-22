// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2020 The Compose Specification Authors.
// Copyright 2022 Unikraft UG. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loader

import (
	"fmt"
	"strings"

	interp "github.com/compose-spec/compose-go/interpolation"
	"github.com/compose-spec/compose-go/template"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"go.unikraft.io/kit/pkg/unikraft/lib"
	"go.unikraft.io/kit/pkg/unikraft/target"
	"go.unikraft.io/kit/schema"
	"go.unikraft.io/kit/schema/types"
)

const (
	KraftProjectName = "KRAFT_PROJECT_NAME"
)

// Load reads a ConfigDetails and returns a fully loaded configuration
func Load(configDetails types.ConfigDetails, options ...func(*LoaderOptions)) (*types.Project, error) {
	if len(configDetails.ConfigFiles) < 1 {
		return nil, errors.Errorf("No files specified")
	}

	opts := &LoaderOptions{
		Interpolate: &interp.Options{
			Substitute:      template.Substitute,
			LookupValue:     configDetails.LookupEnv,
			TypeCastMapping: interpolateTypeCastMapping,
		},
	}

	for _, op := range options {
		op(opts)
	}

	var configs []*types.Config
	for i, file := range configDetails.ConfigFiles {
		configDict := file.Config
		if configDict == nil {
			dict, err := parseConfig(file.Content, opts)
			if err != nil {
				return nil, err
			}
			configDict = dict
			file.Config = dict
			configDetails.ConfigFiles[i] = file
		}

		if !opts.SkipValidation {
			if err := schema.Validate(configDict); err != nil {
				return nil, err
			}
		}

		configDict = groupXFieldsIntoExtensions(configDict)

		cfg, err := loadSections(file.Filename, configDict, configDetails, opts)
		if err != nil {
			return nil, err
		}

		configs = append(configs, cfg)
	}

	model, err := merge(configs)
	if err != nil {
		return nil, err
	}

	projectName, projectNameImperativelySet := opts.GetProjectName()
	model.Name = normalizeProjectName(model.Name)
	if !projectNameImperativelySet && model.Name != "" {
		projectName = model.Name
	}

	if projectName != "" {
		configDetails.Environment[KraftProjectName] = projectName
	}
	project := &types.Project{
		Name:        projectName,
		WorkingDir:  configDetails.WorkingDir,
		Libraries:   model.Libraries,
		Targets:     model.Targets,
		Environment: configDetails.Environment,
		Extensions:  model.Extensions,
	}

	if !opts.SkipNormalization {
		err = normalize(project, opts.ResolvePaths)
		if err != nil {
			return nil, err
		}
	}

	return project, nil
}

func loadSections(filename string, config map[string]interface{}, configDetails types.ConfigDetails, opts *LoaderOptions) (*types.Config, error) {
	var err error
	cfg := types.Config{
		Filename: filename,
	}

	name := ""
	if n, ok := config["name"]; ok {
		name, ok = n.(string)
		if !ok {
			return nil, errors.New("project name must be a string")
		}
	}
	cfg.Name = name

	cfg.Unikraft, err = LoadUnikraft(getSection(config, "unikraft"))
	if err != nil {
		return nil, err
	}

	cfg.Libraries, err = LoadLibraries(getSectionMap(config, "libraries"))
	if err != nil {
		return nil, err
	}

	cfg.Targets, err = LoadTargets(getSectionList(config, "targets"))
	if err != nil {
		return nil, err
	}

	extensions := getSectionMap(config, "extensions")
	if len(extensions) > 0 {
		cfg.Extensions = extensions
	}

	return &cfg, nil
}

// LoadUnikraft produces a UnikraftConfig from a kraft file Dict the source Dict
// is not validated if directly used. Use Load() to enable validation
func LoadUnikraft(source interface{}) (types.UnikraftConfig, error) {
	unikraft := types.UnikraftConfig{}
	err := Transform(source, &unikraft)
	if err != nil {
		return unikraft, err
	}

	return unikraft, nil
}

// LoadLibraries produces a LibraryConfig map from a kraft file Dict the source
// Dict is not validated if directly used. Use Load() to enable validation
func LoadLibraries(source map[string]interface{}) (map[string]lib.LibraryConfig, error) {
	libraries := make(map[string]lib.LibraryConfig)
	if err := Transform(source, &libraries); err != nil {
		return libraries, err
	}

	for name, library := range libraries {
		switch {
		case library.Name == "":
			library.Name = name
		}

		libraries[name] = library
	}

	return libraries, nil
}

// LoadTargets produces a LibraryConfig map from a kraft file Dict the source
// Dict is not validated if directly used. Use Load() to enable validation
func LoadTargets(source []interface{}) ([]target.TargetConfig, error) {
	targets := []target.TargetConfig{}
	if err := Transform(source, &targets); err != nil {
		return targets, err
	}

	return targets, nil
}

func getSection(config map[string]interface{}, key string) interface{} {
	section, ok := config[key]
	if !ok {
		return nil
	}

	return section
}

func getSectionMap(config map[string]interface{}, key string) map[string]interface{} {
	section, ok := config[key]
	if !ok {
		return make(map[string]interface{})
	}

	return section.(map[string]interface{})
}

func getSectionList(config map[string]interface{}, key string) []interface{} {
	section, ok := config[key]
	if !ok {
		return nil
	}

	return section.([]interface{})
}

func parseConfig(b []byte, opts *LoaderOptions) (map[string]interface{}, error) {
	yml, err := ParseYAML(b)
	if err != nil {
		return nil, err
	}
	if !opts.SkipInterpolation {
		return interp.Interpolate(yml, *opts.Interpolate)
	}
	return yml, err
}

// ParseYAML reads the bytes from a file, parses the bytes into a mapping
// structure, and returns it.
func ParseYAML(source []byte) (map[string]interface{}, error) {
	var cfg interface{}
	if err := yaml.Unmarshal(source, &cfg); err != nil {
		return nil, err
	}

	cfgMap, ok := cfg.(map[interface{}]interface{})
	if !ok {
		return nil, errors.Errorf("Top-level object must be a mapping")
	}

	converted, err := convertToStringKeysRecursive(cfgMap, "")
	if err != nil {
		return nil, err
	}

	return converted.(map[string]interface{}), nil
}

func formatInvalidKeyError(keyPrefix string, key interface{}) error {
	var location string
	if keyPrefix == "" {
		location = "at top level"
	} else {
		location = fmt.Sprintf("in %s", keyPrefix)
	}

	return errors.Errorf("Non-string key %s: %#v", location, key)
}

// keys need to be converted to strings for jsonschema
func convertToStringKeysRecursive(value interface{}, keyPrefix string) (interface{}, error) {
	if mapping, ok := value.(map[interface{}]interface{}); ok {
		dict := make(map[string]interface{})
		for key, entry := range mapping {
			str, ok := key.(string)
			if !ok {
				return nil, formatInvalidKeyError(keyPrefix, key)
			}

			var newKeyPrefix string
			if keyPrefix == "" {
				newKeyPrefix = str
			} else {
				newKeyPrefix = fmt.Sprintf("%s.%s", keyPrefix, str)
			}

			convertedEntry, err := convertToStringKeysRecursive(entry, newKeyPrefix)
			if err != nil {
				return nil, err
			}

			dict[str] = convertedEntry
		}

		return dict, nil
	}

	if list, ok := value.([]interface{}); ok {
		var convertedList []interface{}
		for index, entry := range list {
			newKeyPrefix := fmt.Sprintf("%s[%d]", keyPrefix, index)
			convertedEntry, err := convertToStringKeysRecursive(entry, newKeyPrefix)
			if err != nil {
				return nil, err
			}

			convertedList = append(convertedList, convertedEntry)
		}

		return convertedList, nil
	}

	return value, nil
}

func groupXFieldsIntoExtensions(dict map[string]interface{}) map[string]interface{} {
	extras := map[string]interface{}{}
	for key, value := range dict {
		if strings.HasPrefix(key, "x-") {
			extras[key] = value
			delete(dict, key)
		}

		if d, ok := value.(map[string]interface{}); ok {
			dict[key] = groupXFieldsIntoExtensions(d)
		}
	}

	if len(extras) > 0 {
		dict["extensions"] = extras
	}
	return dict
}
