// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package menu

import (
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli"
	"kraftkit.sh/make"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

type Menu struct {
	Architecture string `long:"arch" short:"m" usage:"Filter prepare based on a target's architecture"`
	Platform     string `long:"plat" short:"p" usage:"Filter prepare based on a target's platform"`
	Target       string `long:"target" short:"t" usage:"Filter prepare based on a specific target"`
}

func New() *cobra.Command {
	return cmdfactory.New(&Menu{}, cobra.Command{
		Short:   "Open's Unikraft configuration editor TUI",
		Use:     "menu [DIR]",
		Aliases: []string{"m", "menuconfig"},
		Args:    cmdfactory.MaxDirArgs(1),
		Long: heredoc.Doc(`
			Open Unikraft's configuration editor TUI`),
		Example: heredoc.Doc(`
			# Open configuration editor in the cwd project
			$ kraft menu
			
			# Open configuration editor for a project at a path
			$ kraft menu path/to/app`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
}

func (*Menu) Pre(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

	return nil
}

func (opts *Menu) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	workdir := ""

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	// Initialize at least the configuration options for a project
	project, err := app.NewProjectFromOptions(
		ctx,
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
	)
	if err != nil {
		return err
	}

	// Filter project targets by any provided CLI options
	targets := cli.FilterTargets(
		project.Targets(),
		opts.Architecture,
		opts.Platform,
		opts.Target,
	)

	var t target.Target

	switch {
	case len(targets) == 1:
		t = targets[0]

	case config.G[config.KraftKit](ctx).NoPrompt:
		return fmt.Errorf("could not determine which target to prepare")

	default:
		t, err = cli.SelectTarget(targets)
		if err != nil {
			return err
		}
	}

	return project.Make(
		ctx,
		t,
		make.WithTarget("menuconfig"),
	)
}
