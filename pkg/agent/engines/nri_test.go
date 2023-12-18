/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package engines

import "testing"

func Test_startOOBEngine(t *testing.T) {
	type args struct {
		reason error
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startOOBEngine(tt.args.reason)
		})
	}
}
