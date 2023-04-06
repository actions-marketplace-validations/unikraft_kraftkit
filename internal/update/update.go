// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"kraftkit.sh/internal/version"
	"kraftkit.sh/iostreams"

	"github.com/MakeNowJust/heredoc"
	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
	"kraftkit.sh/log"
)

const KraftKitLatestPath = "https://get.kraftkit.sh/latest.txt"

func Check(ctx context.Context) error {
	if version.Version() == "" {
		return nil
	}

	client := &http.Client{}

	get, err := http.NewRequest("GET", KraftKitLatestPath, nil)
	if err != nil {
		return err
	}

	get.Header.Set("User-Agent", version.Version())
	log.G(ctx).WithFields(logrus.Fields{
		"url":    KraftKitLatestPath,
		"method": "GET",
	}).Trace("http")

	resp, err := client.Do(get)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return err
	}

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	latestVer, err := semver.NewVersion(strings.Split(string(contents), "-")[0])
	if err != nil {
		return err
	}

	currentVer, err := semver.NewVersion(strings.Split(version.Version(), "-")[0])
	if err != nil {
		return err
	}

	if currentVer.LessThan(latestVer) {
		fmt.Fprint(iostreams.G(ctx).Out, heredoc.Docf(`A new version of KraftKit is now available (v%s)!

Please update KraftKit through your local package manager or run:

  curl --proto '=https' --tlsv1.2 -sSf https://get.kraftkit.sh | sh

Read the full changelog:

  https://github.com/unikraft/kraftkit/releases/tag/v%s

To turn off this check, set:

  export KRAFTKIT_NO_CHECK_UPDATES=true

Or use the globally accessible flag '--no-check-updates'.

`,
			string(contents), string(contents)))
	}

	return nil
}
