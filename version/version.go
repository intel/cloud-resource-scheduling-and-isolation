/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package version

import "runtime"

var (
	Version   = "1.0"
	GoVersion = runtime.Version()
)
