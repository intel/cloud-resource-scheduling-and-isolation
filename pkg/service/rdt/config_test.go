/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_modifyRDTClassL3schema(t *testing.T) {
	err := newMockMountsFile()
	if err != nil {
		t.Fatalf("failed to create new mock mounts file: %v", err)
	}
	if resctrlL3Info, err = getRdtL3Info(); err != nil {
		t.Fatalf("Failed to get rdt l3 info: %v", err)
	}
	type args struct {
		className string
		l3        string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "modify rdt class l3 schema failed, class name error",
			args: args{
				className: "guarant",
				l3:        "0xfff",
			},
			wantErr: true,
		},
		{
			name: "modify rdt class l3 schema failed, l3 error",
			args: args{
				className: "guaranteed",
				l3:        "0xf0f",
			},
			wantErr: true,
		},
		{
			name: "modify rdt class l3 schema success 1",
			args: args{
				className: "guaranteed",
				l3:        "0xfff",
			},
			wantErr: false,
		},
		{
			name: "modify rdt class l3 schema success 2",
			args: args{
				className: "guaranteed",
				l3:        "30%",
			},
			wantErr: false,
		},
		{
			name: "modify rdt class l3 schema success 3",
			args: args{
				className: "guaranteed",
				l3:        "9-11",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := modifyRDTClassL3schema(tt.args.className, tt.args.l3); (err != nil) != tt.wantErr {
				t.Errorf("modifyRDTClassL3schema() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_listStrToArray(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name    string
		args    args
		want    []int
		wantErr bool
	}{
		{
			name: "list to array success 1",
			args: args{
				str: "1-2",
			},
			want:    []int{1, 2},
			wantErr: false,
		},
		{
			name: "list to array success 2",
			args: args{
				str: "1-2,3-4",
			},
			want:    []int{1, 2, 3, 4},
			wantErr: false,
		},
		{
			name: "list to array success 3",
			args: args{
				str: "1,3,4",
			},
			want:    []int{1, 3, 4},
			wantErr: false,
		},
		{
			name: "list to array success 4",
			args: args{
				str: "0-2,7-8",
			},
			want:    []int{0, 1, 2, 7, 8},
			wantErr: false,
		},
		{
			name: "list to array failed 1",
			args: args{
				str: "0-2,7-x",
			},
			want:    []int{0, 1, 2},
			wantErr: true,
		},
		{
			name: "list to array failed 2",
			args: args{
				str: "0-2,7-6",
			},
			want:    []int{0, 1, 2},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := listStringToArray(tt.args.str)
			if (err != nil) != tt.wantErr {
				t.Errorf("listStrToArray() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("listStrToArray() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheAllocation(t *testing.T) {
	err := newMockMountsFile()
	if err != nil {
		t.Fatalf("failed to create new mock mounts file: %v", err)
	}
	if resctrlL3Info, err = getRdtL3Info(); err != nil {
		t.Fatalf("Failed to get rdt l3 info: %v", err)
	}

	if res, err := (catPercentageRangeAllocation{lowRange: 0, highRange: 100}).Transform(4, 0xff00); err != nil {
		t.Errorf("unexpected error when overlaying catPctAllocation: %v", err)
	} else if res != 0xff00 {
		t.Errorf("expected 0xff00 but got %#x when overlaying catPctAllocation", res)
	}
	if res, err := (catPercentageRangeAllocation{lowRange: 99, highRange: 100}).Transform(4, 0xff00); err != nil {
		t.Errorf("unexpected error when overlaying catPctAllocation: %v", err)
	} else if res != 0xf000 {
		t.Errorf("expected 0xf000 but got %#x when overlaying catPctAllocation", res)
	}
	if res, err := (catPercentageRangeAllocation{lowRange: 0, highRange: 1}).Transform(4, 0xff00); err != nil {
		t.Errorf("unexpected error when overlaying catPctAllocation: %v", err)
	} else if res != 0xf00 {
		t.Errorf("expected 0xf00 but got %#x when overlaying catPctAllocation", res)
	}
	if res, err := (catPercentageRangeAllocation{lowRange: 20, highRange: 30}).Transform(4, 0x3ff00); err != nil {
		t.Errorf("unexpected error when overlaying catPctAllocation: %v", err)
	} else if res != 0xf00 {
		t.Errorf("expected 0xf00 but got %#x when overlaying catPctAllocation", res)
	}
	if res, err := (catPercentageRangeAllocation{lowRange: 30, highRange: 60}).Transform(4, 0xf00); err != nil {
		t.Errorf("unexpected error when overlaying catPctAllocation: %v", err)
	} else if res != 0xf00 {
		t.Errorf("expected 0xf00 but got %#x when overlaying catPctAllocation", res)
	}
	if _, err := (catPercentageRangeAllocation{lowRange: 0, highRange: 100}).Transform(4, 0); err == nil {
		t.Errorf("unexpected success when overlaying catPctAllocation of invalid percentage range")
	}
	if _, err := (catPercentageRangeAllocation{lowRange: 19, highRange: 5}).Transform(2, 0xff00); err == nil {
		t.Errorf("unexpected success when overlaying catPctAllocation of invalid percentage range")
	}

	abs := catAbsoluteAllocation(0x7)

	if _, err := catAbsoluteAllocation(0x2).Transform(2, 0x20); err == nil {
		t.Errorf("unexpected success when overlaying catAbsoluteAllocation with too small basemask")
	}
	if _, err := abs.Transform(1, 0); err == nil {
		t.Errorf("unexpected success when overlaying catAbsoluteAllocation with empty basemask")
	}
	if _, err := abs.Transform(1, 0x90); err == nil {
		t.Errorf("unexpected success when overlaying too wide catAbsoluteAllocation")
	}
	if res, err := abs.Transform(1, 0xf00); err != nil {
		t.Errorf("unexpected error when overlaying catAbsoluteAllocation: %v", err)
	} else if res != 0x700 {
		t.Errorf("expected 0x700 but got %#x when overlaying catAbsoluteAllocation", res)
	}
	if _, err := abs.Transform(1, 0xf03); err == nil {
		t.Errorf("unexpected success when overlaying catAbsoluteAllocation with non-contiguous basemask")
	}

}

func TestCacheProportion(t *testing.T) {
	if _, err := CacheProportion("1").transform(2); err == nil {
		t.Errorf("unexpected success when parsing bitmask cache allocation")
	}
	if _, err := CacheProportion("3-x").transform(2); err == nil {
		t.Errorf("unexpected success when parsing bitmask cache allocation")
	}
	if a, err := CacheProportion("3,4,5-7").transform(2); err != nil {
		t.Errorf("unexpected error when parsing cache allocation: %v", err)
	} else if a != catAbsoluteAllocation(0xf8) {
		t.Errorf("expected 0xf8 but got %#x", a)
	}
	if _, err := CacheProportion("20-10%").transform(2); err == nil {
		t.Errorf("unexpected success when parsing percentage range cache allocation")
	}
	if _, err := CacheProportion("1-1f%").transform(2); err == nil {
		t.Errorf("unexpected success when parsing percentage range cache allocation")
	}
	if _, err := CacheProportion("1-101%").transform(2); err == nil {
		t.Errorf("unexpected success when parsing percentage range cache allocation")
	}
	if a, err := CacheProportion("10-20%").transform(2); err != nil {
		t.Errorf("unexpected error when parsing cache allocation: %v", err)
	} else if a != (catPercentageRangeAllocation{lowRange: 10, highRange: 20}) {
		t.Errorf("expected {10 20} but got %v", a)
	}

	if _, err := CacheProportion("a%").transform(2); err == nil {
		t.Errorf("unexpected success when parsing percentage cache allocation")
	}
	if _, err := CacheProportion("181%").transform(2); err == nil {
		t.Errorf("unexpected success when parsing percentage cache allocation")
	}
	if a, err := CacheProportion("80%").transform(2); err != nil {
		t.Errorf("unexpected error when parsing cache allocation: %v", err)
	} else if a != catPercentageAllocation(80) {
		t.Errorf("expected 10%% but got %d%%", a)
	}

	if _, err := CacheProportion("0xx").transform(2); err == nil {
		t.Errorf("unexpected success when parsing bitmask cache allocation")
	}
	if _, err := CacheProportion("0x11").transform(2); err == nil {
		t.Errorf("unexpected success when parsing bitmask cache allocation")
	}
	if a, err := CacheProportion("0x70").transform(2); err != nil {
		t.Errorf("unexpected error when parsing cache allocation: %v", err)
	} else if a != catAbsoluteAllocation(0x70) {
		t.Errorf("expected 0x70 but got %#x", a)
	}
}

func TestListStrToArray(t *testing.T) {
	testSet := map[string][]int{
		"":              {},
		"0":             {0},
		"1":             {1},
		"5-7":           {5, 6, 7},
		"1,3,5":         {1, 3, 5},
		"4,2,0,6,10,8":  {0, 2, 4, 6, 8, 10},
		"1,3-4,10-12,8": {1, 3, 4, 8, 10, 11, 12},
	}
	for str, expected := range testSet {
		ans, err := listStringToArray(str)
		if err != nil {
			t.Errorf("when converting %q: %v, unexpected error", str, err)
		}
		if !cmp.Equal(ans, expected) {
			t.Errorf("from %q expected %v, got %v", str, expected, ans)
		}
	}

	// Negative test cases
	negTestSet := []string{
		"13-13",
		"14-13",
		"a-2",
		"b",
		"3-c",
		"1,2,,3",
		"1,2,3-",
		",",
		"-",
		"1,",
		"256",
		"256-257",
		"0-256",
		",12",
		"-4",
		"0-",
	}
	for _, s := range negTestSet {
		a, err := listStringToArray(s)
		if err == nil {
			t.Errorf("expected err but got %v when converting %q", a, s)
		}
	}
}
