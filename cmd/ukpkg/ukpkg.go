// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
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

package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/config"
	"kraftkit.sh/schema"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/internal/logger"

	"kraftkit.sh/initrd"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/ui/processtree"
	"kraftkit.sh/unikraft/target"

	"kraftkit.sh/cmd/ukpkg/list"
	"kraftkit.sh/cmd/ukpkg/source"
	"kraftkit.sh/cmd/ukpkg/update"

	// Additional initializers
	_ "kraftkit.sh/manifest"
)

type ukpkgOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams

	// Command-line arguments
	Format       string
	ForceFormat  bool
	Architecture string
	Platform     string
	Kernel       string
	DotConfig    string
	Target       string
	Initrd       string
	Volumes      []string
	KernelDbg    bool
	WithDbg      bool
}

func main() {
	f := cmdfactory.New(
		cmdfactory.WithPackageManager(),
	)

	cmd, err := cmdutil.NewCmd(f, "ukpkg",
		cmdutil.WithSubcmds(
			list.ListCmd(f),
			source.SourceCmd(f),
			update.UpdateCmd(f),
		),
	)
	if err != nil {
		panic("could not initialize 'ukpkg' commmand")
	}

	opts := &ukpkgOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	cmd.Short = "Package and distribute Unikraft unikernels and their dependencies"
	cmd.Use = "ukpkg [FLAGS] [SUBCOMMAND|DIR]"
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Docf(`
		Package and distribute Unikraft unikernels and their dependencies.

		With %[1]sukpkg%[1]s you are able to turn output artifacts from %[1]sukbuild%[1]s
		into a distributable archive ready for deployment.  At the same time,
		%[1]sukpkg%[1]s allows you to manage these archives: pulling, pushing, or
		adding them to a project.

		The default behaviour of %[1]sukpkg%[1]s is to package a project.  Given no
		arguments, you will be guided through interactive mode.

		For initram and disk images, passing in a directory as the argument will
		result automatically packaging that directory into the requested format.
		Separating the input with a %[1]s:%[1]s delimeter allows you to set the
		output that of the artifact.
	`, "`")
	cmd.Example = heredoc.Doc(`
		# Package the current Unikraft project (cwd)
		$ ukpkg

		# Package path to a Unikraft project
		$ ukpkg path/to/application

		# Package with an additional initramfs
		$ ukpkg --initrd ./root-fs .

		# Same as above but also save the resulting CPIO artifact locally
		$ ukpkg --initrd ./root-fs:./root-fs.cpio .
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if (len(opts.Architecture) > 0 || len(opts.Platform) > 0) && len(opts.Target) > 0 {
			return fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
		}

		var err error
		var workdir string
		if len(args) == 0 {
			workdir, err = os.Getwd()
			if err != nil {
				return err
			}
		} else {
			workdir = args[0]
		}

		return pkgRun(opts, workdir)
	}

	// TODO: Enable flag if multiple managers are detected?
	cmd.Flags().StringVarP(
		&opts.Format,
		"as", "M",
		"auto",
		"Force the packaging handler format (omittion will attempt auto-detect)",
	)

	cmd.Flags().BoolVar(
		&opts.ForceFormat,
		"force-format",
		false,
		"Force the use of a packaging handler format",
	)

	cmd.Flags().StringVarP(
		&opts.Architecture,
		"arch", "m",
		"",
		"Filter the creation of the package by architecture of known targets",
	)

	cmd.Flags().StringVarP(
		&opts.Platform,
		"plat", "p",
		"",
		"Filter the creation of the package by platform of known targets",
	)

	cmd.Flags().StringVarP(
		&opts.Kernel,
		"kernel", "k",
		"",
		"Override the path to the unikernel image",
	)

	cmd.Flags().StringVarP(
		&opts.DotConfig,
		"config", "c",
		"",
		"Override the path to the KConfig `.config` file",
	)

	cmd.Flags().BoolVar(
		&opts.KernelDbg,
		"dbg",
		false,
		"Package the debuggable (symbolic) kernel image instead of the stripped image",
	)

	cmd.Flags().BoolVar(
		&opts.WithDbg,
		"with-dbg",
		false,
		"In addition to the stripped kernel, include the debug image",
	)

	cmd.Flags().StringVarP(
		&opts.Target,
		"target", "t",
		"",
		"Package a particular known target",
	)

	cmd.Flags().StringVarP(
		&opts.Initrd,
		"initrd", "i",
		"",
		"Path to init ramdisk to bundle within the package (passing a path will "+
			"automatically generate a CPIO image)",
	)

	cmd.Flags().StringSliceVarP(
		&opts.Volumes,
		"volumes", "v",
		[]string{},
		"Additional volumes to bundle within the package",
	)

	os.Exit(int(cmdutil.Execute(f, cmd)))
}

func pkgRun(opts *ukpkgOptions, workdir string) error {
	var err error

	pm, err := opts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := opts.Logger()
	if err != nil {
		return err
	}

	// Force a particular package manager
	if len(opts.Format) > 0 && opts.Format != "auto" {
		pm, err = pm.From(opts.Format)
		if err != nil {
			return err
		}
	}

	projectOpts, err := schema.NewProjectOptions(
		nil,
		schema.WithLogger(plog),
		schema.WithWorkingDirectory(workdir),
		schema.WithDefaultConfigPath(),
		schema.WithPackageManager(&pm),
		schema.WithResolvedPaths(true),
		schema.WithDotConfig(true),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	app, err := schema.NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	var packages []pack.Package

	// Generate a package for every matching requested target
	for _, targ := range app.Targets {
		switch true {
		case
			// If no arguments are supplied
			len(opts.Target) == 0 &&
				len(opts.Architecture) == 0 &&
				len(opts.Platform) == 0,

			// If the --target flag is supplied and the target name match
			len(opts.Target) > 0 &&
				targ.Name() == opts.Target,

			// If only the --arch flag is supplied and the target's arch matches
			len(opts.Architecture) > 0 &&
				len(opts.Platform) == 0 &&
				targ.Architecture.Name() == opts.Architecture,

			// If only the --plat flag is supplied and the target's platform matches
			len(opts.Platform) > 0 &&
				len(opts.Architecture) == 0 &&
				targ.Platform.Name() == opts.Platform,

			// If both the --arch and --plat flag are supplied and match the target
			len(opts.Platform) > 0 &&
				len(opts.Architecture) > 0 &&
				targ.Architecture.Name() == opts.Architecture &&
				targ.Platform.Name() == opts.Platform:

			packs, err := initPackage(app.Name(), targ, projectOpts, pm, opts)
			if err != nil {
				return fmt.Errorf("could not create package: %s", err)
			}

			packages = append(packages, packs...)

		default:
			continue
		}
	}

	if len(packages) == 0 {
		plog.Info("nothing to package")
		return nil
	}

	cfgm, err := opts.ConfigManager()
	if err != nil {
		return err
	}

	parallel := !cfgm.Config.NoParallel
	norender := logger.LoggerTypeFromString(cfgm.Config.Log.Type) != logger.FANCY
	if norender {
		parallel = false
	} else {
		plog.SetOutput(ioutil.Discard)
	}

	var tree []*processtree.ProcessTreeItem
	for _, p := range packages {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		p := p

		tree = append(tree, processtree.NewProcessTreeItem(
			"Packaging "+p.CanonicalName(),
			p.Options().ArchPlatString(),
			func(l log.Logger) error {
				// Apply the incoming logger which is tailored to display as a
				// sub-terminal within the fancy processtree.
				p.ApplyOptions(
					pack.WithLogger(l),
				)

				return p.Pack()
			},
		))
	}

	model, err := processtree.NewProcessTree(
		[]processtree.ProcessTreeOption{
			processtree.WithVerb("Packaging"),
			processtree.IsParallel(parallel),
			processtree.WithRenderer(norender),
			processtree.WithLogger(plog),
		},
		tree...,
	)
	if err != nil {
		return err
	}

	if err := model.Start(); err != nil {
		return err
	}

	return nil
}

func initPackage(name string,
	targ target.TargetConfig,
	projectOpts *schema.ProjectOptions,
	pm packmanager.PackageManager,
	opts *ukpkgOptions,
) ([]pack.Package, error) {
	var err error

	log, err := opts.Logger()
	if err != nil {
		return nil, err
	}

	log.Tracef("initializing package")

	// Path to the kernel image
	kernel := opts.Kernel
	if len(kernel) == 0 {
		kernel = targ.Kernel
	}

	// Prefer the debuggable (symbolic) kernel as the main kernel
	if opts.KernelDbg && !opts.WithDbg {
		kernel = targ.KernelDbg
	}

	extraPackOpts := []pack.PackageOption{
		pack.WithName(targ.Name()),
		pack.WithArchitecture(targ.Architecture.Name()),
		pack.WithPlatform(targ.Platform.Name()),
		pack.WithKernel(kernel),
	}

	// Options for the initramfs if set
	initrdConfig := targ.Initrd
	if len(opts.Initrd) > 0 {
		initrdConfig, err = initrd.ParseInitrdConfig(projectOpts.WorkingDir, opts.Initrd)
		if err != nil {
			return nil, fmt.Errorf("could not parse --initrd flag with value %s: %s", opts.Initrd, err)
		}
	}

	// Warn if potentially missing configuration options
	// if initrdConfig != nil && unikraft.EnabledInitramfs()
	extraPackOpts = append(extraPackOpts,
		pack.WithInitrdConfig(initrdConfig),
	)

	packOpts, err := pack.NewPackageOptions(extraPackOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not prepare package for target: %s: %v", targ.Name(), err)
	}

	// Switch the package manager the desired format for this target
	if len(targ.Format) > 0 && targ.Format != "auto" {
		if pm.String() == "umbrella" {
			pm, err = pm.From(targ.Format)
			if err != nil {
				return nil, err
			}

			// Skip this target as we cannot package it
		} else if pm.String() != targ.Format && !opts.ForceFormat {
			log.Warn("skipping %s target %s", targ.Format, targ.Name)
			return nil, nil
		}
	}

	pack, err := pm.NewPackageFromOptions(packOpts)
	if err != nil {
		return nil, fmt.Errorf("could not initialize package: %s", err)
	}

	return pack, nil
}
