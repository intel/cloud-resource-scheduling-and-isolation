/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package hostpath

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"k8s.io/utils/mount"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent/csi/state"
	"sigs.k8s.io/IOIsolation/pkg/agent/disk"
)

func Test_nodeServer_NodePublishVolume(t *testing.T) {
	type args struct {
		ctx context.Context
		req *csi.NodePublishVolumeRequest
	}
	patch1 := gomonkey.ApplyFunc(mount.IsNotMountPoint, func(_ mount.Interface, t string) (bool, error) {
		if t == "/tmp/abcdef" {
			return true, nil
		} else {
			return false, nil
		}
	})
	defer patch1.Reset()

	patch2 := gomonkey.ApplyFunc(disk.GetMountPath, func(devid string) (string, error) {
		if devid == "test" {
			return "/tmp", nil
		} else {
			return "", fmt.Errorf("failed to get mount path")
		}
	})
	defer patch2.Reset()

	statefile, err := state.New(filepath.Join(CsiDir, "state.json"))
	if err != nil {
		t.Error(err)
	}
	vol1 := state.Volume{
		VolID:         volumeId,
		VolName:       "test",
		VolSize:       102400,
		VolPath:       "/tmp/567890",
		VolAccessType: state.MountAccess,
		Kind:          "",
		Attached:      true,
		Staged:        []string{},
	}
	vol2 := state.Volume{
		VolID:         "abcde",
		VolName:       "test1",
		VolSize:       102400,
		VolPath:       "/tmp/abcde",
		VolAccessType: state.BlockAccess,
		Kind:          "",
		Attached:      true,
		Staged:        []string{"/tmp/abcde"},
	}
	vol3 := state.Volume{
		VolID:         "abcdef",
		VolName:       "test1",
		VolSize:       102400,
		VolPath:       "/tmp/abcdef",
		VolAccessType: state.MountAccess,
		Kind:          "",
		Attached:      true,
		Staged:        []string{"/tmp/abcdef"},
	}
	if err := statefile.UpdateVolume(vol1); err != nil {
		t.Error(err)
	}
	if err := statefile.UpdateVolume(vol2); err != nil {
		t.Error(err)
	}
	if err := statefile.UpdateVolume(vol3); err != nil {
		t.Error(err)
	}
	defer os.Remove(filepath.Join(CsiDir, "state.json"))

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.NodePublishVolumeResponse
		wantErr bool
	}{
		{
			name: "empty reqs",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid reqs: empty volume capability",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeCapability: nil,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid reqs: empty volume id",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
					VolumeId: "",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid reqs: empty target path",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
					VolumeId:   "123",
					TargetPath: "",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "volume id not found",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId:   "xyz",
					TargetPath: "/tmp/123",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid reqs: empty staged",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId:   volumeId,
					TargetPath: "/tmp/123",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid reqs: mismatched staged",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId:   "abcde",
					TargetPath: "/tmp/abcde",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
					StagingTargetPath: "/tmp/abcde",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid reqs: staged elsewhere",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId:   "abcde",
					TargetPath: "/tmp/abcde",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
					StagingTargetPath: "/tmp/xyz",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid reqs: cannot publish a non-mount volume",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId:   "abcde",
					TargetPath: "/tmp/abcde",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
					StagingTargetPath: "/tmp/abcde",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "already mounted",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId:   "abcdef",
					TargetPath: "/tmp/abcde",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
					StagingTargetPath: "/tmp/abcdef",
				},
			},
			want:    &csi.NodePublishVolumeResponse{},
			wantErr: false,
		},
		{
			name: "cannot get mount path with device ID",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId:   "abcdef",
					TargetPath: "/tmp/abcdef",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
					VolumeContext:     map[string]string{utils.AnnoDeviceID: "test2"},
					StagingTargetPath: "/tmp/abcdef",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "success",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId:   "abcdef",
					TargetPath: "/tmp/abcdef",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
					VolumeContext:     map[string]string{utils.AnnoDeviceID: "test"},
					StagingTargetPath: "/tmp/abcdef",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &hostPath{
				state: statefile,
			}
			got, err := ns.NodePublishVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("nodeServer.NodePublishVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nodeServer.NodePublishVolume() = %v, want %v", got, tt.want)
			}
		})
	}
	if err := statefile.DeleteVolume(volumeId); err != nil {
		t.Error(err)
	}
}

func Test_nodeServer_NodeUnPublishVolume(t *testing.T) {
	patch1 := gomonkey.ApplyFunc(mount.IsNotMountPoint, func(_ mount.Interface, t string) (bool, error) {
		return true, nil
	})
	defer patch1.Reset()
	type args struct {
		ctx context.Context
		req *csi.NodeUnpublishVolumeRequest
	}

	statefile, err := state.New(filepath.Join(CsiDir, "state.json"))
	if err != nil {
		t.Error(err)
	}
	vol1 := state.Volume{
		VolID:         volumeId,
		VolName:       "test",
		VolSize:       102400,
		VolPath:       "/tmp/567890",
		VolAccessType: state.MountAccess,
		Kind:          "",
		Attached:      true,
		Published:     []string{"/tmp/456"},
	}
	if err := statefile.UpdateVolume(vol1); err != nil {
		t.Error(err)
	}

	tests := []struct {
		name    string
		args    args
		want    *csi.NodeUnpublishVolumeResponse
		wantErr bool
	}{
		{
			name: "empty reqs",
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid reqs: empty volume id",
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{
					VolumeId: "",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid reqs: empty target path",
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{
					VolumeId:   "123",
					TargetPath: "",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "volume id not found",
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{
					VolumeId:   "xyz",
					TargetPath: "/tmp/123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "volume is not published",
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{
					VolumeId:   volumeId,
					TargetPath: "/tmp/123",
				},
			},
			want:    &csi.NodeUnpublishVolumeResponse{},
			wantErr: false,
		},
		{
			name: "success",
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{
					VolumeId:   volumeId,
					TargetPath: "/tmp/456",
				},
			},
			want:    &csi.NodeUnpublishVolumeResponse{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &hostPath{
				state: statefile,
			}
			got, err := ns.NodeUnpublishVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("nodeServer.NodeUnpublishVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nodeServer.NodeUnpublishVolume() = %v, want %v", got, tt.want)
			}
		})
	}
	if err := statefile.DeleteVolume(volumeId); err != nil {
		t.Error(err)
	}
}

func Test_nodeServer_NodeGetCapabilities(t *testing.T) {
	type args struct {
		ctx context.Context
		req *csi.NodeGetCapabilitiesRequest
	}
	tests := []struct {
		name    string
		args    args
		want    *csi.NodeGetCapabilitiesResponse
		wantErr bool
	}{
		{
			name: "success",
			args: args{
				ctx: context.Background(),
				req: &csi.NodeGetCapabilitiesRequest{},
			},
			want: &csi.NodeGetCapabilitiesResponse{
				Capabilities: []*csi.NodeServiceCapability{
					{
						Type: &csi.NodeServiceCapability_Rpc{
							Rpc: &csi.NodeServiceCapability_RPC{
								Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
							},
						},
					},
					{
						Type: &csi.NodeServiceCapability_Rpc{
							Rpc: &csi.NodeServiceCapability_RPC{
								Type: csi.NodeServiceCapability_RPC_VOLUME_CONDITION,
							},
						},
					},
					{
						Type: &csi.NodeServiceCapability_Rpc{
							Rpc: &csi.NodeServiceCapability_RPC{
								Type: csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
							},
						},
					},
					{
						Type: &csi.NodeServiceCapability_Rpc{
							Rpc: &csi.NodeServiceCapability_RPC{
								Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := &hostPath{}
			got, err := cs.NodeGetCapabilities(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("controllerServer.ControllerGetCapabilities() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("controllerServer.ControllerGetCapabilities() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsMountedElsewhere(t *testing.T) {
	type args struct {
		req *csi.NodePublishVolumeRequest
		vol state.Volume
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "mounted elsewhere",
			args: args{
				req: &csi.NodePublishVolumeRequest{
					TargetPath: "/tmp/123",
				},
				vol: state.Volume{
					Published: []string{"/tmp/123"},
				},
			},
			want: false,
		},
		{
			name: "not mounted elsewhere",
			args: args{
				req: &csi.NodePublishVolumeRequest{
					TargetPath: "/tmp/123",
				},
				vol: state.Volume{
					Published: []string{"/tmp/456"},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMountedElsewhere(tt.args.req, tt.args.vol); got != tt.want {
				t.Errorf("isMountedElsewhere() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasSingleNodeSingleWriterAccessMode(t *testing.T) {
	tests := []struct {
		name string
		args *csi.NodePublishVolumeRequest
		want bool
	}{
		{
			name: "single write access node",
			args: &csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER,
					},
				},
			},
			want: true,
		},
		{
			name: "unknown access mode",
			args: &csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasSingleNodeSingleWriterAccessMode(tt.args); got != tt.want {
				t.Errorf("hasSingleNodeSingleWriterAccessMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakefile(t *testing.T) {
	tmpFile := "/tmp/mkftest"
	err := makeFile(tmpFile)
	defer os.Remove(tmpFile)

	assert.Nil(t, err)
	assert.FileExists(t, tmpFile)
}

func TestNodeExpandVolume(t *testing.T) {
	hp := &hostPath{}
	ret, err := hp.NodeExpandVolume(context.Background(), &csi.NodeExpandVolumeRequest{})
	assert.Nil(t, err)
	assert.Equal(t, &csi.NodeExpandVolumeResponse{}, ret)
}

func TestNodeGetInfo(t *testing.T) {
	config1 := Config{
		NodeID:            "testNode1",
		MaxVolumesPerNode: 100,
	}
	config2 := Config{
		NodeID:            "testNode2",
		MaxVolumesPerNode: 100,
		EnableTopology:    true,
	}
	config3 := Config{
		NodeID:            "testNode3",
		MaxVolumesPerNode: 100,
		EnableTopology:    false,
		AttachLimit:       10,
	}
	tests := []struct {
		name string
		args *hostPath
		want *csi.NodeGetInfoResponse
	}{
		{
			name: "case 1",
			args: &hostPath{
				config: config1,
			},
			want: &csi.NodeGetInfoResponse{
				NodeId:            "testNode1",
				MaxVolumesPerNode: 100,
			},
		},
		{
			name: "case 2",
			args: &hostPath{
				config: config2,
			},
			want: &csi.NodeGetInfoResponse{
				NodeId:            "testNode2",
				MaxVolumesPerNode: 100,
				AccessibleTopology: &csi.Topology{
					Segments: map[string]string{TopologyKeyNode: "testNode2"},
				},
			},
		},
		{
			name: "case 3",
			args: &hostPath{
				config: config3,
			},
			want: &csi.NodeGetInfoResponse{
				NodeId:            "testNode3",
				MaxVolumesPerNode: 10,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if resp, _ := tt.args.NodeGetInfo(context.Background(), &csi.NodeGetInfoRequest{}); !reflect.DeepEqual(resp, tt.want) {
				t.Errorf("nodeserver.NodeGetInfo() = %v, want %v", resp, tt.want)
			}
		})
	}

}

func TestNodeGetVolumeStats(t *testing.T) {
	patch1 := gomonkey.ApplyFunc(getPVStats, func(_ string) (int64, int64, int64, int64, int64, int64, error) {
		return 100, 1000, 900, 12345, 11234, 11111, nil
	})
	defer patch1.Reset()
	s, err := state.New(path.Join("/tmp", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	hp := &hostPath{
		state: s,
	}
	patch2 := gomonkey.ApplyFunc(checkMountPointExist, func(_ string) (bool, error) {
		return true, nil
	})
	defer patch2.Reset()

	volume := state.Volume{
		VolID:   "ngvtest",
		VolName: "ngvtest",
	}
	if err := hp.state.UpdateVolume(volume); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path.Join("/tmp", "state.json"))
	tests := []struct {
		name   string
		args   *csi.NodeGetVolumeStatsRequest
		want   *csi.NodeGetVolumeStatsResponse
		errMsg string
	}{
		{
			name:   "empty volume id",
			args:   &csi.NodeGetVolumeStatsRequest{},
			want:   nil,
			errMsg: "Volume ID not provided",
		},
		{
			name: "empty volume path",
			args: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "123",
				VolumePath: "",
			},
			want:   nil,
			errMsg: "Volume Path not provided",
		},
		{
			name: "volume does not exist in state file",
			args: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "123",
				VolumePath: "456",
			},
			want:   nil,
			errMsg: "does not exist in the volumes list",
		},
		{
			name: "volume exists",
			args: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "ngvtest",
				VolumePath: "/tmp/NodeGetVolumeStatsTest",
			},
			want:   nil,
			errMsg: "Could not get file information from",
		},
		{
			name: "success",
			args: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "ngvtest",
				VolumePath: "/tmp",
			},
			want: &csi.NodeGetVolumeStatsResponse{
				Usage: []*csi.VolumeUsage{
					{
						Available: 100,
						Used:      900,
						Total:     1000,
						Unit:      csi.VolumeUsage_BYTES,
					}, {
						Available: 11234,
						Used:      11111,
						Total:     12345,
						Unit:      csi.VolumeUsage_INODES,
					},
				},
				VolumeCondition: &csi.VolumeCondition{},
			},
			errMsg: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := hp.NodeGetVolumeStats(context.Background(), tt.args)
			if err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("nodeserver.NodeGetVolumeStats() error = %v, wantErr %v", err, tt.errMsg)
				return
			}
			if !reflect.DeepEqual(resp, tt.want) {
				t.Errorf("nodeserver.NodeGetVolumeStats() = %v, want %v", resp, tt.want)
			}
		})
	}
}

func TestNodeStageVolume(t *testing.T) {
	s, err := state.New(path.Join("/tmp", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	hp := &hostPath{
		state: s,
	}

	volume := state.Volume{
		VolID:     "nsvtest",
		VolName:   "nsvtest",
		Attached:  false,
		Staged:    []string{"456"},
		Published: []string{"789"},
	}
	volume1 := state.Volume{
		VolID:   "nsvtest1",
		VolName: "nsvtest1",
	}
	if err := hp.state.UpdateVolume(volume); err != nil {
		t.Fatal(err)
	}
	if err := hp.state.UpdateVolume(volume1); err != nil {
		t.Fatal(err)
	}

	defer os.Remove(path.Join("/tmp", "state.json"))
	type args struct {
		req  *csi.NodeStageVolumeRequest
		conf *Config
	}
	tests := []struct {
		name   string
		args   *args
		want   *csi.NodeStageVolumeResponse
		errMsg string
	}{
		{
			name: "empty volume id",
			args: &args{
				req: &csi.NodeStageVolumeRequest{},
			},
			want:   nil,
			errMsg: "Volume ID missing in request",
		},
		{
			name: "empty target path",
			args: &args{
				req: &csi.NodeStageVolumeRequest{
					VolumeId: "123",
				},
			},
			want:   nil,
			errMsg: "Target path missing in request",
		},
		{
			name: "empty volume capacity",
			args: &args{
				req: &csi.NodeStageVolumeRequest{
					VolumeId:          "123",
					StagingTargetPath: "456",
				},
			},
			want:   nil,
			errMsg: "Volume Capability missing in request",
		},
		{
			name: "volume does not exist in state file",
			args: &args{
				req: &csi.NodeStageVolumeRequest{
					VolumeId:          "123",
					StagingTargetPath: "456",
					VolumeCapability:  &csi.VolumeCapability{},
				},
			},
			want:   nil,
			errMsg: "does not exist in the volumes list",
		},
		{
			name: "sequence error",
			args: &args{
				req: &csi.NodeStageVolumeRequest{
					VolumeId:          "nsvtest",
					StagingTargetPath: "456",
					VolumeCapability:  &csi.VolumeCapability{},
				},
				conf: &Config{
					EnableAttach: true,
				},
			},
			want:   nil,
			errMsg: "ControllerPublishVolume must be called on volume",
		},
		{
			name: "already staged",
			args: &args{
				req: &csi.NodeStageVolumeRequest{
					VolumeId:          "nsvtest",
					StagingTargetPath: "456",
					VolumeCapability:  &csi.VolumeCapability{},
				},
			},
			want:   &csi.NodeStageVolumeResponse{},
			errMsg: "nothing to do",
		},
		{
			name: "already staged",
			args: &args{
				req: &csi.NodeStageVolumeRequest{
					VolumeId:          "nsvtest",
					StagingTargetPath: "789",
					VolumeCapability:  &csi.VolumeCapability{},
				},
			},
			want:   nil,
			errMsg: "is already staged at",
		},
		{
			name: "success",
			args: &args{
				req: &csi.NodeStageVolumeRequest{
					VolumeId:          "nsvtest1",
					StagingTargetPath: "789",
					VolumeCapability:  &csi.VolumeCapability{},
				},
			},
			want:   &csi.NodeStageVolumeResponse{},
			errMsg: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.conf != nil {
				hp.config = *tt.args.conf
			} else {
				hp.config = Config{}
			}

			resp, err := hp.NodeStageVolume(context.Background(), tt.args.req)
			if err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("nodeserver.NodeStageVolume() error = %v, wantErr %v", err, tt.errMsg)
				return
			}
			if !reflect.DeepEqual(resp, tt.want) {
				t.Errorf("nodeserver.NodeStageVolume() = %v, want %v", resp, tt.want)
			}
		})
	}
}

func TestNodeUnstageVolume(t *testing.T) {
	s, err := state.New(path.Join("/tmp", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	hp := &hostPath{
		state: s,
	}

	volume := state.Volume{
		VolID:     "nuvtest",
		VolName:   "nuvtest",
		Published: []string{"789"},
		Staged:    []string{"789"},
	}
	volume1 := state.Volume{
		VolID:   "nsvtest1",
		VolName: "nsvtest1",
		Staged:  []string{"789"},
	}
	if err := hp.state.UpdateVolume(volume); err != nil {
		t.Fatal(err)
	}
	if err := hp.state.UpdateVolume(volume1); err != nil {
		t.Fatal(err)
	}

	defer os.Remove(path.Join("/tmp", "state.json"))
	tests := []struct {
		name   string
		args   *csi.NodeUnstageVolumeRequest
		want   *csi.NodeUnstageVolumeResponse
		errMsg string
	}{
		{
			name:   "empty volume id",
			args:   &csi.NodeUnstageVolumeRequest{},
			want:   nil,
			errMsg: "Volume ID missing in request",
		},
		{
			name: "empty target path",
			args: &csi.NodeUnstageVolumeRequest{
				VolumeId: "123",
			},
			want:   nil,
			errMsg: "Target path missing in request",
		},
		{
			name: "volume does not exist in state file",
			args: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "123",
				StagingTargetPath: "456",
			},
			want:   nil,
			errMsg: "does not exist in the volumes list",
		},
		{
			name: "nothing to do",
			args: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "nuvtest",
				StagingTargetPath: "456",
			},
			want:   &csi.NodeUnstageVolumeResponse{},
			errMsg: "nothing to do",
		},
		{
			name: "published",
			args: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "nuvtest",
				StagingTargetPath: "789",
			},
			want:   nil,
			errMsg: "is still published at",
		},
		{
			name: "success",
			args: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "nuvtest1",
				StagingTargetPath: "789",
			},
			want:   &csi.NodeUnstageVolumeResponse{},
			errMsg: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := hp.NodeUnstageVolume(context.Background(), tt.args)
			if err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("nodeserver.NodeUnstageVolume() error = %v, wantErr %v", err, tt.errMsg)
				return
			}
			if resp != nil && !reflect.DeepEqual(resp, tt.want) {
				t.Errorf("nodeserver.NodeUnstageVolume() = %v, want %v", resp, tt.want)
			}
		})
	}
}
