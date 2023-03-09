// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package machine

import (
	"fmt"
	"os"
	"time"
)

// MachineConfig describes an individual virtual machine
type MachineConfig struct {
	// ID is the UUID of the guest.
	ID MachineID `json:"id,omitempty"`

	// Name is the name of the guest.
	Name MachineName `json:"name,omitempty"`

	// Description of the guest.
	Description string `json:"description,omitempty"`

	// Architecture of the machine, e.g.: x86_64, arm64.
	Architecture string `json:"architecture"`

	// Platform of the machine, e.g.: kvm, xen.
	Platform string `json:"platform"`

	// Driver represents the hypervisor once the machine has entered into an
	// instantiated lifecycle.
	DriverName string `json:"driver,omitempty"`

	// Source represents where the machine image derived from
	Source string `json:"source,omitempty"`

	// KernelPath is the guest kernel host path.
	KernelPath string `json:"kernel_path,omitempty"`

	// Arguments are the list of arguments to pass to the kernel
	Arguments []string `json:"arguments,omitempty"`

	// InitrdPath is the guest initrd image host path.
	// ImagePath and InitrdPath cannot be set at the same time.
	InitrdPath string `json:"initrd_path,omitempty"`

	// HardwareAcceleration indicates whether host machine acceleration should be
	// used when available by the underlying driver.
	HardwareAcceleration bool

	// NumVCPUs specifies default number of vCPUs for the VM.
	NumVCPUs uint64 `json:"num_vcpus,omitempty"`

	// MemorySize specifies default memory size in MiB for the VM.
	MemorySize uint64 `json:"mem_size,omitempty"`

	// DestroyOnExit indicates whether the machine should be destroyed once it
	// exists
	DestroyOnExit bool

	// CreatedAt represents when the machine was created with its respected driver
	// or VMM.
	CreatedAt time.Time `json:"created_at"`

	// ExitedAt represents when the machine fully shutdown
	ExitedAt time.Time `json:"exited_at"`

	// ExitStatus represents the error code returned after a machine exits
	ExitStatus int `json:"exit_status"`
}

type MachineOption func(mo *MachineConfig) error

func NewMachineConfig(mopts ...MachineOption) (*MachineConfig, error) {
	mcfg := &MachineConfig{}

	for _, o := range mopts {
		if err := o(mcfg); err != nil {
			return nil, err
		}
	}

	// None of these are available options to MachineConfig, so set sensible
	// defaults
	mcfg.CreatedAt = time.Time{}
	mcfg.ExitedAt = time.Time{}
	mcfg.ExitStatus = -1

	return mcfg, nil
}

func WithID(id MachineID) MachineOption {
	return func(mo *MachineConfig) error {
		mo.ID = id
		return nil
	}
}

func WithName(name MachineName) MachineOption {
	return func(mo *MachineConfig) error {
		mo.Name = name
		return nil
	}
}

func WithDescription(description string) MachineOption {
	return func(mo *MachineConfig) error {
		mo.Description = description
		return nil
	}
}

func WithArchitecture(arch string) MachineOption {
	return func(mo *MachineConfig) error {
		mo.Architecture = arch
		return nil
	}
}

func WithPlatform(plat string) MachineOption {
	return func(mo *MachineConfig) error {
		mo.Platform = plat
		return nil
	}
}

func WithDriverName(driver string) MachineOption {
	return func(mo *MachineConfig) error {
		mo.DriverName = driver
		return nil
	}
}

func WithSource(source string) MachineOption {
	return func(mo *MachineConfig) error {
		mo.Source = source
		return nil
	}
}

func WithKernel(kernel string) MachineOption {
	return func(mo *MachineConfig) error {
		f, err := os.Stat(kernel)
		if err != nil {
			return err
		} else if f.Size() == 0 || f.IsDir() {
			return fmt.Errorf("invalid kernel: %s", kernel)
		}

		mo.KernelPath = kernel
		return nil
	}
}

func WithArguments(arguments []string) MachineOption {
	return func(mo *MachineConfig) error {
		mo.Arguments = arguments
		return nil
	}
}

func WithInitRd(initrd string) MachineOption {
	return func(mo *MachineConfig) error {
		mo.InitrdPath = initrd
		return nil
	}
}

func WithAcceleration(hwAccel bool) MachineOption {
	return func(mo *MachineConfig) error {
		mo.HardwareAcceleration = hwAccel
		return nil
	}
}

func WithNumVCPUs(numVCPUs uint64) MachineOption {
	return func(mo *MachineConfig) error {
		mo.NumVCPUs = numVCPUs
		return nil
	}
}

func WithMemorySize(memorySize uint64) MachineOption {
	return func(mo *MachineConfig) error {
		mo.MemorySize = memorySize
		return nil
	}
}

func WithDestroyOnExit(destroyOnExit bool) MachineOption {
	return func(mo *MachineConfig) error {
		mo.DestroyOnExit = destroyOnExit
		return nil
	}
}
