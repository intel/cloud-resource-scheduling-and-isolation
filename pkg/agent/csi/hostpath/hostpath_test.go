/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package hostpath

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	"sigs.k8s.io/IOIsolation/pkg/agent/csi/state"
)

func TestNewHostPathDriver(t *testing.T) {
	type args struct {
		cfg        Config
		coreClient kubernetes.Interface
		vClient    versioned.Interface
		state      state.State
	}
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()
	statefile, err := state.New(filepath.Join(CsiDir, "state.json"))
	if err != nil {
		t.Error(err)
	}
	volume := state.Volume{
		VolID:         volumeId,
		VolName:       "test",
		VolSize:       102400,
		VolPath:       "/tmp/567890",
		VolAccessType: state.MountAccess,
		Kind:          "",
		Attached:      true,
	}
	if err := statefile.UpdateVolume(volume); err != nil {
		t.Error(err)
	}

	tests := []struct {
		name    string
		args    args
		want    *hostPath
		wantErr bool
	}{
		{
			name: "valid config",
			args: args{cfg: Config{
				DriverName:           "localstorage.csi.k8s.io",
				Endpoint:             "unix:///csi/csi.sock",
				NodeID:               NodeName,
				EnableTopology:       true,
				StateDir:             CsiDir,
				MaxVolumeSize:        1024 * 1024 * 1024 * 1024,
				CheckVolumeLifecycle: true,
			},
				coreClient: fakeKubeClient,
				state:      statefile,
			},
			want: &hostPath{
				config: Config{
					DriverName:           "localstorage.csi.k8s.io",
					Endpoint:             "unix:///csi/csi.sock",
					NodeID:               NodeName,
					EnableTopology:       true,
					StateDir:             CsiDir,
					MaxVolumeSize:        1024 * 1024 * 1024 * 1024,
					CheckVolumeLifecycle: true,
				},
				coreClient: fakeKubeClient,
				state:      statefile,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewHostPathDriver(tt.args.cfg, tt.args.coreClient, tt.args.vClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewHostPathDriver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewHostPathDriver() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hostPath_deleteVolume(t *testing.T) {
	type fields struct {
		config     Config
		coreClient kubernetes.Interface
		// mutex      sync.Mutex
		state state.State
	}
	type args struct {
		volID string
	}
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()

	statefile, err := state.New(filepath.Join(CsiDir, "state.json"))
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(filepath.Join(CsiDir, "state.json"))
	testfields := fields{
		config: Config{
			NodeID:               NodeName,
			EnableTopology:       true,
			StateDir:             CsiDir,
			MaxVolumeSize:        1024 * 1024 * 1024 * 1024,
			CheckVolumeLifecycle: true,
		},
		coreClient: fakeKubeClient,
		state:      statefile,
	}
	vol1 := state.Volume{
		VolID:         volumeId,
		VolName:       "test",
		VolSize:       102400,
		VolPath:       "/tmp/567890",
		VolAccessType: state.MountAccess,
		Kind:          "",
		Attached:      true,
		DeviceID:      "",
	}
	vol2 := state.Volume{
		VolID:         "dvtest",
		VolName:       "test",
		VolSize:       102400,
		VolPath:       "/tmp/dvtest",
		VolAccessType: state.MountAccess,
		Kind:          "",
		Attached:      true,
		DeviceID:      "test",
	}
	if err := statefile.UpdateVolume(vol1); err != nil {
		t.Error(err)
	}
	if err := statefile.UpdateVolume(vol2); err != nil {
		t.Error(err)
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:   "not existing volume id",
			fields: testfields,
			args: args{
				volID: "unknown",
			},
			wantErr: false,
		},
		{
			name:   "empty device id",
			fields: testfields,
			args: args{
				volID: volumeId,
			},
			wantErr: true,
		},
		{
			name:   "failed to the mount path",
			fields: testfields,
			args: args{
				volID: "dvtest",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hp := &hostPath{
				config:     tt.fields.config,
				coreClient: tt.fields.coreClient,
				state:      tt.fields.state,
			}
			if err := hp.deleteVolume(tt.args.volID); (err != nil) != tt.wantErr {
				t.Errorf("hostPath.deleteVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
