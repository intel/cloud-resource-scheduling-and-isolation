/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"testing"

	"github.com/vishvananda/netlink"
)

func Test_createIfb(t *testing.T) {
	type args struct {
		ifbDeviceName string
		mtu           int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			"Test1",
			args{"testIfb", 1500},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createIfb(tt.args.ifbDeviceName, tt.args.mtu)
		})
	}

}

func Test_addIfbLimit(t *testing.T) {
	type args struct {
		bw  uint64
		ifb string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			"Test1",
			args{100, "testIfb"},
		},
		{
			"Test2",
			args{100, "testIfb1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addIfbLimit(tt.args.bw, tt.args.ifb)
		})
	}
}

func Test_delQdiscIfExist(t *testing.T) {
	type args struct {
		linkIndex int
		tpe       string
	}
	ifbD, _ := netlink.LinkByName("testIfb")
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"Test1",
			args{ifbD.Attrs().Index, "tbf"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := delQdiscIfExist(tt.args.linkIndex, tt.args.tpe); (err != nil) != tt.wantErr {
				t.Errorf("delQdiscIfExist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_operateIngressQdisc(t *testing.T) {
	type args struct {
		hostDevice netlink.Link
		op         int32
	}
	ifbD, _ := netlink.LinkByName("testIfb")
	tests := []struct {
		name string
		args args
	}{
		{
			"Test1",
			args{ifbD, OP_ADD},
		},
		{
			"Test2",
			args{ifbD, OP_DEL},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operateIngressQdisc(tt.args.hostDevice, tt.args.op)
		})
	}
}

func Test_convertIPToUint32(t *testing.T) {
	type args struct {
		ip_str string
	}
	tests := []struct {
		name    string
		args    args
		want    uint32
		wantErr bool
	}{
		{
			"Test1",
			args{"10.244.0.55"},
			183762999,
			false,
		},
		{
			"Test2",
			args{"10.239.241.96"},
			183497056,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertIPToUint32(tt.args.ip_str)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertIPToUint32() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("convertIPToUint32() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mbitToBit(t *testing.T) {
	type args struct {
		bw uint64
	}
	tests := []struct {
		name string
		args args
		want uint64
	}{
		{
			"Test1",
			args{30},
			30000000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mbitToBit(tt.args.bw); got != tt.want {
				t.Errorf("mbitToBit() = %v, want %v", got, tt.want)
			}
		})
	}
}
