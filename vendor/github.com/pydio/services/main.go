/*
 * Copyright 2007-2017 Abstrium <contact (at) pydio.com>
 * This file is part of Pydio.
 *
 * Pydio is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Pydio is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with Pydio.  If not, see <http://www.gnu.org/licenses/>.
 *
 * The latest code can be found at <https://pydio.com/>.
 */

package main // import "github.com/pydio/services"

import (
	"fmt"
	"os"
	"runtime"

	version "github.com/hashicorp/go-version"
	"github.com/minio/mc/pkg/console"
	pydio "github.com/pydio/services/cmd"
)

const (
	// Pydio requires at least Go v1.8
	minGoVersion        = "1.8"
	goVersionConstraint = ">= " + minGoVersion
)

// Check if this binary is compiled with at least minimum Go version.
func checkGoVersion(goVersionStr string) error {
	constraint, err := version.NewConstraint(goVersionConstraint)
	if err != nil {
		return fmt.Errorf("'%s': %s", goVersionConstraint, err)
	}

	goVersion, err := version.NewVersion(goVersionStr)
	if err != nil {
		return err
	}

	if !constraint.Check(goVersion) {
		return fmt.Errorf("Pydio is not compiled by Go %s.  Please recompile accordingly.",
			goVersionConstraint)
	}

	return nil
}

func main() {
	// When `go get` is used minimum Go version check is not triggered but it would have compiled it successfully.
	// However such binary will fail at runtime, hence we also check Go version at runtime.
	if err := checkGoVersion(runtime.Version()[2:]); err != nil {
		console.Fatalln("Go runtime version check failed.", err)
	}

	pydio.Main(os.Args)
}
