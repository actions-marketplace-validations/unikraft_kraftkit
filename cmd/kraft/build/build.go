// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/internal/cli"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"

	"kraftkit.sh/tui/processtree"
)

type Build struct {
	Architecture string `long:"arch" short:"m" usage:"Filter the creation of the build by architecture of known targets"`
	DotConfig    string `long:"config" short:"c" usage:"Override the path to the KConfig .config file"`
	Fast         bool   `long:"fast" usage:"Use maximum parallelization when performing the build"`
	Jobs         int    `long:"jobs" short:"j" usage:"Allow N jobs at once"`
	KernelDbg    bool   `long:"dbg" usage:"Build the debuggable (symbolic) kernel image instead of the stripped image"`
	NoCache      bool   `long:"no-cache" short:"F" usage:"Force a rebuild even if existing intermediate artifacts already exist"`
	NoConfigure  bool   `long:"no-configure" usage:"Do not run Unikraft's configure step before building"`
	NoFetch      bool   `long:"no-fetch" usage:"Do not run Unikraft's fetch step before building"`
	NoPrepare    bool   `long:"no-prepare" usage:"Do not run Unikraft's prepare step before building"`
	NoPull       bool   `long:"no-pull" usage:"Do not pull packages before invoking Unikraft's build system"`
	Platform     string `long:"plat" short:"p" usage:"Filter the creation of the build by platform of known targets"`
	SaveBuildLog string `long:"build-log" usage:"Use the specified file to save the output from the build"`
	Target       string `long:"target" short:"t" usage:"Build a particular known target"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Build{}, cobra.Command{
		Short: "Configure and build Unikraft unikernels",
		Use:   "build [FLAGS] [SUBCOMMAND|DIR]",
		Args:  cmdfactory.MaxDirArgs(1),
		Long: heredoc.Docf(`
			Build a Unikraft unikernel.

			The default behaviour of %[1]skraft build%[1]s is to build a project.  Given no
			arguments, you will be guided through interactive mode.
		`, "`"),
		Example: heredoc.Doc(`
			# Build the current project (cwd)
			$ kraft build

			# Build path to a Unikraft project
			$ kraft build path/to/app`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (*Build) Pre(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

	return nil
}

func (opts *Build) pull(ctx context.Context, project app.Application, workdir string, norender bool, nameWidth int) error {
	var missingPacks []pack.Package
	var processes []*paraprogress.Process
	var searches []*processtree.ProcessTreeItem
	parallel := !config.G[config.KraftKit](ctx).NoParallel
	auths := config.G[config.KraftKit](ctx).Auth

	if _, err := project.Components(ctx); err != nil && project.Template().Name() != "" {
		var packages []pack.Package
		search := processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s",
				unikraft.TypeNameVersion(project.Template()),
			), "",
			func(ctx context.Context) error {
				packages, err = packmanager.G(ctx).Catalog(ctx,
					packmanager.WithName(project.Template().Name()),
					packmanager.WithTypes(unikraft.ComponentTypeApp),
					packmanager.WithVersion(project.Template().Version()),
					packmanager.WithCache(!opts.NoCache),
					packmanager.WithAuthConfig(config.G[config.KraftKit](ctx).Auth),
				)
				if err != nil {
					return err
				}

				if len(packages) == 0 {
					return fmt.Errorf("could not find: %s",
						unikraft.TypeNameVersion(project.Template()),
					)
				} else if len(packages) > 1 {
					return fmt.Errorf("too many options for %s",
						unikraft.TypeNameVersion(project.Template()),
					)
				}

				return nil
			},
		)

		treemodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(parallel),
				processtree.WithRenderer(norender),
				processtree.WithFailFast(true),
			},
			search,
		)
		if err != nil {
			return err
		}

		if err := treemodel.Start(); err != nil {
			return fmt.Errorf("could not complete search: %v", err)
		}

		proc := paraprogress.NewProcess(
			fmt.Sprintf("pulling %s",
				unikraft.TypeNameVersion(packages[0]),
			),
			func(ctx context.Context, w func(progress float64)) error {
				return packages[0].Pull(
					ctx,
					pack.WithPullProgressFunc(w),
					pack.WithPullWorkdir(workdir),
					// pack.WithPullChecksum(!opts.NoChecksum),
					pack.WithPullCache(!opts.NoCache),
				)
			},
		)

		processes = append(processes, proc)

		paramodel, err := paraprogress.NewParaProgress(
			ctx,
			processes,
			paraprogress.IsParallel(parallel),
			paraprogress.WithRenderer(norender),
			paraprogress.WithFailFast(true),
			paraprogress.WithNameWidth(nameWidth),
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return fmt.Errorf("could not pull all components: %v", err)
		}
	}

	if project.Template().Name() != "" {
		templateWorkdir, err := unikraft.PlaceComponent(workdir, project.Template().Type(), project.Template().Name())
		if err != nil {
			return err
		}

		templateProject, err := app.NewProjectFromOptions(
			ctx,
			app.WithProjectWorkdir(templateWorkdir),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		project, err = templateProject.MergeTemplate(ctx, project)
		if err != nil {
			return err
		}
	}

	// Overwrite template with user options
	components, err := project.Components(ctx)
	if err != nil {
		return err
	}
	for _, component := range components {
		component := component // loop closure
		auths := auths

		searches = append(searches, processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s",
				unikraft.TypeNameVersion(component),
			), "",
			func(ctx context.Context) error {
				p, err := packmanager.G(ctx).Catalog(ctx,
					packmanager.WithName(component.Name()),
					packmanager.WithTypes(component.Type()),
					packmanager.WithVersion(component.Version()),
					packmanager.WithSource(component.Source()),
					packmanager.WithCache(!opts.NoCache),
					packmanager.WithAuthConfig(auths),
				)
				if err != nil {
					return err
				}

				if len(p) == 0 {
					return fmt.Errorf("could not find: %s",
						unikraft.TypeNameVersion(component),
					)
				} else if len(p) > 1 {
					return fmt.Errorf("too many options for %s",
						unikraft.TypeNameVersion(component),
					)
				}

				missingPacks = append(missingPacks, p...)
				return nil
			},
		))
	}

	if len(searches) > 0 {
		treemodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(parallel),
				processtree.WithRenderer(norender),
				processtree.WithFailFast(true),
			},
			searches...,
		)
		if err != nil {
			return err
		}

		if err := treemodel.Start(); err != nil {
			return fmt.Errorf("could not complete search: %v", err)
		}
	}

	if len(missingPacks) > 0 {
		for _, p := range missingPacks {
			p := p // loop closure
			auths := auths
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("pulling %s",
					unikraft.TypeNameVersion(p),
				),
				func(ctx context.Context, w func(progress float64)) error {
					return p.Pull(
						ctx,
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(workdir),
						// pack.WithPullChecksum(!opts.NoChecksum),
						pack.WithPullCache(!opts.NoCache),
						pack.WithPullAuthConfig(auths),
					)
				},
			))
		}

		paramodel, err := paraprogress.NewParaProgress(
			ctx,
			processes,
			paraprogress.IsParallel(parallel),
			paraprogress.WithRenderer(norender),
			paraprogress.WithFailFast(true),
			paraprogress.WithNameWidth(nameWidth),
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return fmt.Errorf("could not pull all components: %v", err)
		}
	}

	return nil
}

func (opts *Build) Run(cmd *cobra.Command, args []string) error {
	var err error
	var workdir string

	if (len(opts.Architecture) > 0 || len(opts.Platform) > 0) && len(opts.Target) > 0 {
		return fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
	}

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	ctx := cmd.Context()

	// Initialize at least the configuration options for a project
	project, err := app.NewProjectFromOptions(
		ctx,
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
	)
	if err != nil {
		return err
	}

	if !app.IsWorkdirInitialized(workdir) {
		return fmt.Errorf("cannot build uninitialized project")
	}

	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY
	nameWidth := -1

	// Calculate the width of the longest process name so that we can align the
	// two independent processtrees if we are using "render" mode (aka the fancy
	// mode is enabled).
	if !norender {
		// The longest word is "configuring" (which is 11 characters long), plus
		// additional space characters (2 characters), brackets (2 characters) the
		// name of the project and the target/plat string (which is variable in
		// length).
		for _, targ := range project.Targets() {
			if newLen := len(targ.Name()) + len(target.TargetPlatArchName(targ)) + 15; newLen > nameWidth {
				nameWidth = newLen
			}
		}
	}

	if !opts.NoPull {
		if err := opts.pull(ctx, project, workdir, norender, nameWidth); err != nil {
			return err
		}
	}

	processes := []*paraprogress.Process{} // reset

	// Filter project targets by any provided CLI options
	selected := cli.FilterTargets(
		project.Targets(),
		opts.Architecture,
		opts.Platform,
		opts.Target,
	)

	if len(selected) == 0 {
		return fmt.Errorf("no targets selected to build")
	}

	var mopts []make.MakeOption
	if opts.Jobs > 0 {
		mopts = append(mopts, make.WithJobs(opts.Jobs))
	} else {
		mopts = append(mopts, make.WithMaxJobs(opts.Fast))
	}

	for _, targ := range selected {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ
		if !opts.NoConfigure {
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("configuring %s (%s)", targ.Name(), target.TargetPlatArchName(targ)),
				func(ctx context.Context, w func(progress float64)) error {
					return project.Configure(
						ctx,
						targ, // Target-specific options
						nil,  // No extra configuration options
						make.WithProgressFunc(w),
						make.WithSilent(true),
						make.WithExecOptions(
							exec.WithStdin(iostreams.G(ctx).In),
							exec.WithStdout(log.G(ctx).Writer()),
							exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
						),
					)
				},
			))
		}

		if !opts.NoPrepare {
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("preparing %s (%s)", targ.Name(), target.TargetPlatArchName(targ)),
				func(ctx context.Context, w func(progress float64)) error {
					return project.Prepare(
						ctx,
						targ, // Target-specific options
						append(
							mopts,
							make.WithProgressFunc(w),
							make.WithExecOptions(
								exec.WithStdout(log.G(ctx).Writer()),
								exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
							),
						)...,
					)
				},
			))
		}

		processes = append(processes, paraprogress.NewProcess(
			fmt.Sprintf("building %s (%s)", targ.Name(), target.TargetPlatArchName(targ)),
			func(ctx context.Context, w func(progress float64)) error {
				return project.Build(
					ctx,
					targ, // Target-specific options
					app.WithBuildProgressFunc(w),
					app.WithBuildMakeOptions(append(mopts,
						make.WithExecOptions(
							exec.WithStdout(log.G(ctx).Writer()),
							exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
						),
					)...),
					app.WithBuildLogFile(opts.SaveBuildLog),
				)
			},
		))
	}

	paramodel, err := paraprogress.NewParaProgress(
		ctx,
		processes,
		// Disable parallelization as:
		//  - The first process may be pulling the container image, which is
		//    necessary for the subsequent build steps;
		//  - The Unikraft build system can re-use compiled files from previous
		//    compilations (if the architecture does not change).
		paraprogress.IsParallel(false),
		paraprogress.WithRenderer(norender),
		paraprogress.WithFailFast(true),
	)
	if err != nil {
		return err
	}

	return paramodel.Start()
}
