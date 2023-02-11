// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package unikraft

import (
	"fmt"
	"path/filepath"
	"regexp"
)

type ComponentType string

const (
	ComponentTypeUnknown ComponentType = "unknown"
	ComponentTypeCore    ComponentType = "core"
	ComponentTypeArch    ComponentType = "arch"
	ComponentTypePlat    ComponentType = "plat"
	ComponentTypeLib     ComponentType = "lib"
	ComponentTypeApp     ComponentType = "app"
)

func ComponentTypes() map[string]ComponentType {
	return map[string]ComponentType{
		"core":  ComponentTypeCore,
		"arch":  ComponentTypeArch,
		"archs": ComponentTypeArch,
		"plat":  ComponentTypePlat,
		"plats": ComponentTypePlat,
		"lib":   ComponentTypeLib,
		"libs":  ComponentTypeLib,
		"app":   ComponentTypeApp,
		"apps":  ComponentTypeApp,
	}
}

func (ct ComponentType) Plural() string {
	if ct == ComponentTypeUnknown || ct == ComponentTypeCore {
		return string(ct)
	}

	return string(ct) + "s"
}

// GuessNameAndType attempts to parse the input string, which could be formatted
// such that its type, name and version are present
func GuessTypeNameVersion(input string) (ComponentType, string, string, error) {
	re := regexp.MustCompile(
		`(?i)^` +
			`(?:(?P<type>(?:lib|app|plat|arch)s?)[\-/])?` +
			`(?P<name>[\w\-\_\*]*)` +
			`(?:\:(?P<version>[\w\.\-\_]*))?` +
			`$`,
	)

	match := re.FindStringSubmatch(input)
	if len(match) == 0 {
		return ComponentTypeUnknown, "", "", fmt.Errorf("cannot determine name and type from \"%s\"", input)
	}

	t, n, v := match[1], match[2], match[3]

	if n == "unikraft" {
		t = string(ComponentTypeCore)
	}

	// Is the type recognised?
	if found, ok := ComponentTypes()[t]; ok {
		return found, n, v, nil
	}

	return ComponentTypeUnknown, n, v, nil
}

// PlaceComponent is a universal source of truth for identifying the path to
// place a component
func PlaceComponent(workdir string, t ComponentType, name string) (string, error) {
	// TODO: Should the hidden-file (`.`) be optional?
	switch t {
	case ComponentTypeCore:
		return filepath.Join(workdir, VendorDir, "unikraft"), nil
	case ComponentTypeApp,
		ComponentTypeLib,
		ComponentTypeArch,
		ComponentTypePlat:
		return filepath.Join(workdir, VendorDir, t.Plural(), name), nil
	}

	return "", fmt.Errorf("cannot place component of unknown type")
}
