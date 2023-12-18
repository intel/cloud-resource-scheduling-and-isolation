/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package agent

import (
	"testing"

	utils "sigs.k8s.io/IOIsolation/pkg"
)

func TestPodData_Type(t *testing.T) {
	type fields struct {
		T          int
		Generation int64
		PodEvents  map[string]PodEventInfo
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "test pod data type nri",
			fields: fields{
				T: 0,
			},
			want: EVENT_NRI,
		},
		{
			name: "test pod data type disk",
			fields: fields{
				T: 1,
			},
			want: EVENT_DISK,
		},
		{
			name: "test pod data type disk cr",
			fields: fields{
				T: 2,
			},
			want: EVENT_DISK_CR,
		},
		{
			name: "test pod data type net",
			fields: fields{
				T: 3,
			},
			want: EVENT_NET,
		},
		{
			name: "test pod data type net cr",
			fields: fields{
				T: 4,
			},
			want: EVENT_NET_CR,
		},
		{
			name: "test pod data type rdt cr",
			fields: fields{
				T: 5,
			},
			want: EVENT_RDT_CR,
		},
		{
			name: "test pod data type rdt quantity cr",
			fields: fields{
				T: 6,
			},
			want: EVENT_RDT_Quantity_CR,
		},
		{
			name: "test pod data type none",
			fields: fields{
				T: 7,
			},
			want: EVENT_NONE,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodData{
				T:          tt.fields.T,
				Generation: tt.fields.Generation,
				PodEvents:  tt.fields.PodEvents,
			}
			if got := p.Type(); got != tt.want {
				t.Errorf("PodData.Type() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerData_Type(t *testing.T) {
	type fields struct {
		T              int
		Generation     int64
		ContainerInfos map[string]ContainerInfo
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "test pod data type nri",
			fields: fields{
				T: 0,
			},
			want: EVENT_NRI,
		},
		{
			name: "test pod data type disk",
			fields: fields{
				T: 1,
			},
			want: EVENT_DISK,
		},
		{
			name: "test pod data type disk cr",
			fields: fields{
				T: 2,
			},
			want: EVENT_DISK_CR,
		},
		{
			name: "test pod data type net",
			fields: fields{
				T: 3,
			},
			want: EVENT_NET,
		},
		{
			name: "test pod data type net cr",
			fields: fields{
				T: 4,
			},
			want: EVENT_NET_CR,
		},
		{
			name: "test pod data type rdt cr",
			fields: fields{
				T: 5,
			},
			want: EVENT_RDT_CR,
		},
		{
			name: "test pod data type rdt quantity cr",
			fields: fields{
				T: 6,
			},
			want: EVENT_RDT_Quantity_CR,
		},
		{
			name: "test pod data type none",
			fields: fields{
				T: 7,
			},
			want: EVENT_NONE,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ContainerData{
				T:              tt.fields.T,
				Generation:     tt.fields.Generation,
				ContainerInfos: tt.fields.ContainerInfos,
			}
			if got := p.Type(); got != tt.want {
				t.Errorf("ContainerData.Type() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerData_Expect(t *testing.T) {
	type fields struct {
		T              int
		Generation     int64
		ContainerInfos map[string]ContainerInfo
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name: "test container data expect",
			fields: fields{
				T: 0,
			},
			want: ExpectContainerData,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ContainerData{
				T:              tt.fields.T,
				Generation:     tt.fields.Generation,
				ContainerInfos: tt.fields.ContainerInfos,
			}
			if got := p.Expect(); got != tt.want {
				t.Errorf("ContainerData.Expect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassData_Type(t *testing.T) {
	type fields struct {
		T          int
		ClassInfos map[string]ClassInfo
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "test class data type rdt",
			fields: fields{
				T: 0,
			},
			want: EVENT_RDT,
		},
		{
			name: "test class data type rdt quantity",
			fields: fields{
				T: 1,
			},
			want: EVENT_RDT_Quantity,
		},
		{
			name: "test class data type none",
			fields: fields{
				T: 2,
			},
			want: EVENT_NONE,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ClassData{
				T:          tt.fields.T,
				ClassInfos: tt.fields.ClassInfos,
			}
			if got := p.Type(); got != tt.want {
				t.Errorf("ClassData.Type() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllProfile_Type(t *testing.T) {
	type fields struct {
		DProfile utils.DiskInfos
		NProfile utils.NicInfos
		DInfo    AllDiskInfo
		T        int
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "test profile data type profile disk",
			fields: fields{
				T: 0,
			},
			want: EVENT_PROF_DISK,
		},
		{
			name: "test profile data type profile net",
			fields: fields{
				T: 1,
			},
			want: EVENT_PROF_NET,
		},
		{
			name: "test profile data type none",
			fields: fields{
				T: 2,
			},
			want: EVENT_NONE,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &AllProfile{
				DProfile: tt.fields.DProfile,
				NProfile: tt.fields.NProfile,
				DInfo:    tt.fields.DInfo,
				T:        tt.fields.T,
			}
			if got := p.Type(); got != tt.want {
				t.Errorf("AllProfile.Type() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyConfig_Type(t *testing.T) {
	tests := []struct {
		name string
		p    *PolicyConfig
		want string
	}{
		{
			name: "test policy config data type none",
			p: &PolicyConfig{
				"test": Policy{
					Throttle: "test",
				},
			},
			want: EVENT_POLICY,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Type(); got != tt.want {
				t.Errorf("PolicyConfig.Type() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdminResourceConfig_Type(t *testing.T) {
	type fields struct {
		DiskBePool         int
		DiskGaPool         int
		NetworkBePool      int
		NetworkGaPool      int
		NamespaceWhitelist map[string]struct{}
		Disks              map[string][]string
		SysDiskRatio       int
		Loglevel           IOILogLevel
		Interval           ReportInterval
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "test admin config data type none",
			fields: fields{
				DiskBePool: 0,
			},
			want: EVENT_ADMIN,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &AdminResourceConfig{
				DiskBePool:         tt.fields.DiskBePool,
				DiskSysPool:        tt.fields.DiskGaPool,
				NetBePool:          tt.fields.NetworkBePool,
				NetSysPool:         tt.fields.NetworkGaPool,
				NamespaceWhitelist: tt.fields.NamespaceWhitelist,
				SysDiskRatio:       tt.fields.SysDiskRatio,
				Loglevel:           tt.fields.Loglevel,
				Interval:           tt.fields.Interval,
			}
			if got := p.Type(); got != tt.want {
				t.Errorf("AdminResourceConfig.Type() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodData_Expect(t *testing.T) {
	type fields struct {
		T          int
		Generation int64
		PodEvents  map[string]PodEventInfo
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name: "test pod data expect",
			fields: fields{
				T: 0,
			},
			want: ExpectPodData,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodData{
				T:          tt.fields.T,
				Generation: tt.fields.Generation,
				PodEvents:  tt.fields.PodEvents,
			}
			if got := p.Expect(); got != tt.want {
				t.Errorf("PodData.Expect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassData_Expect(t *testing.T) {
	type fields struct {
		T          int
		ClassInfos map[string]ClassInfo
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name: "test class data expect",
			fields: fields{
				T: 0,
			},
			want: ExpectClassData,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ClassData{
				T:          tt.fields.T,
				ClassInfos: tt.fields.ClassInfos,
			}
			if got := p.Expect(); got != tt.want {
				t.Errorf("ClassData.Expect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyConfig_Expect(t *testing.T) {
	tests := []struct {
		name string
		p    *PolicyConfig
		want int
	}{
		{
			name: "test policy config expect",
			p: &PolicyConfig{
				"test": Policy{
					Throttle: "test",
				},
			},
			want: ExpectPolicy,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Expect(); got != tt.want {
				t.Errorf("PolicyConfig.Expect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllProfile_Expect(t *testing.T) {
	type fields struct {
		DProfile utils.DiskInfos
		NProfile utils.NicInfos
		DInfo    AllDiskInfo
		T        int
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name: "test profile data expect",
			fields: fields{
				T: 0,
			},
			want: ExpectProfile,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &AllProfile{
				DProfile: tt.fields.DProfile,
				NProfile: tt.fields.NProfile,
				DInfo:    tt.fields.DInfo,
				T:        tt.fields.T,
			}
			if got := p.Expect(); got != tt.want {
				t.Errorf("AllProfile.Expect() = %v, want %v", got, tt.want)
			}
		})
	}
}
