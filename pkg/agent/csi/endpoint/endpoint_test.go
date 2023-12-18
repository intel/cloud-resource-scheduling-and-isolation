/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package endpoint

import (
	// "net"
	// "reflect"
	"net"
	"reflect"
	"testing"
)

func TestValidateAndParse(t *testing.T) {
	type args struct {
		ep string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name:    "empty endpoint",
			args:    args{ep: "unix://"},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name:    "valid endpoint",
			args:    args{ep: "unix://10.239.119.20"},
			want:    "unix",
			want1:   "10.239.119.20",
			wantErr: false,
		},
		{
			name:    "default unix endpoint",
			args:    args{ep: "10.239.119.20"},
			want:    "unix",
			want1:   "10.239.119.20",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := ValidateAndParse(tt.args.ep)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Parse() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Parse() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestListen(t *testing.T) {
	type args struct {
		endpoint string
	}
	tests := []struct {
		name    string
		args    args
		want    net.Listener
		want1   func()
		wantErr bool
	}{
		{
			name:    "wrong endpoint",
			args:    args{endpoint: "unix:///"},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := Listen(tt.args.endpoint)
			t.Logf("got1 %v, got %v", &got1, got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Listen() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Listen() got = %v, want %v", got, tt.want)
			}
		})
	}
}
