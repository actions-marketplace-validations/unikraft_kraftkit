// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package platform

import (
	"context"
	"path/filepath"

	zip "api.zip"

	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/machine/firecracker"
	"kraftkit.sh/machine/qemu"
	"kraftkit.sh/machine/store"
)

var qemuV1alpha1Driver = func(ctx context.Context, opts ...any) (machinev1alpha1.MachineService, error) {
	service, err := qemu.NewMachineV1alpha1Service(ctx, opts...)
	if err != nil {
		return nil, err
	}

	embeddedStore, err := store.NewEmbeddedStore[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus](
		filepath.Join(
			config.G[config.KraftKit](ctx).RuntimeDir,
			"machinev1alpha1",
		),
	)
	if err != nil {
		return nil, err
	}

	return machinev1alpha1.NewMachineServiceHandler(
		ctx,
		service,
		zip.WithStore[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus](embeddedStore, zip.StoreRehydrationSpecNil),
		zip.WithBefore(storePlatformFilter(PlatformQEMU)),
	)
}

var firecrackerV1alpha1Driver = func(ctx context.Context, opts ...any) (machinev1alpha1.MachineService, error) {
	if set.NewStringSet("debug", "trace").Contains(config.G[config.KraftKit](ctx).Log.Level) {
		opts = append(opts, firecracker.WithDebug(true))
	}
	service, err := firecracker.NewMachineV1alpha1Service(ctx, opts...)
	if err != nil {
		return nil, err
	}

	embeddedStore, err := store.NewEmbeddedStore[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus](
		filepath.Join(
			config.G[config.KraftKit](ctx).RuntimeDir,
			"machinev1alpha1",
		),
	)
	if err != nil {
		return nil, err
	}

	return machinev1alpha1.NewMachineServiceHandler(
		ctx,
		service,
		zip.WithStore[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus](embeddedStore, zip.StoreRehydrationSpecNil),
		zip.WithBefore(storePlatformFilter(PlatformFirecracker)),
	)
}

// hostSupportedStrategies returns the map of known supported drivers for the
// given host.
func hostSupportedStrategies() map[Platform]*Strategy {
	return map[Platform]*Strategy{
		PlatformQEMU: {
			NewMachineV1alpha1: qemuV1alpha1Driver,
		},
		PlatformKVM: {
			NewMachineV1alpha1: qemuV1alpha1Driver,
		},
		PlatformFirecracker: {
			NewMachineV1alpha1: firecrackerV1alpha1Driver,
		},
	}
}
