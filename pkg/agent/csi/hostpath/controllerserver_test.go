/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package hostpath

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent/csi/state"
)

const (
	deviceId     = "123456"
	deviceName   = "TestDev"
	NodeName     = "testNode"
	NodeNameFail = "testNode1"
	CsiDir       = "/tmp"
	volumeId     = "567890"
)

type fields struct {
	c  kubernetes.Interface
	cf *Config
	s  state.State
}

func createConfigMap(client kubernetes.Interface, ctx context.Context, nodeName string, m map[string]string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.IoiNamespace,
			Name:      fmt.Sprintf("%s-%s", utils.DeviceIDConfigMap, nodeName),
			Labels:    map[string]string{"app": utils.DeviceIDConfigMap},
		},
		Data: m,
	}
	_, err := client.CoreV1().ConfigMaps(utils.IoiNamespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("Failed to create ConfigMap volume-device-map: node=%v, err=%v", NodeName, err)
	}
	return nil
}

func Test_controllerServer_CreateVolume(t *testing.T) {
	type args struct {
		ctx context.Context
		req *csi.CreateVolumeRequest
	}

	ctx := context.Background()
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()

	rName, err := uuid.NewUUID()
	if err != nil {
		t.Error("failed to generate uuid")
	}

	// save disk info in node agent
	// disk.DiskInfos = make(map[string]common.DiskInfo)
	// disk.DiskInfos[deviceID] = common.DiskInfo{
	// 	Name:  deviceName,
	// 	Mount: filepath.Join(CsiDir, deviceName),
	// }

	// add device info in configmap
	mSuccess := map[string]string{
		fmt.Sprintf("%s.%s", "pvcName", "default"): "123456",
	}
	mFail := map[string]string{
		fmt.Sprintf("%s.%s", "pvcName", "default"): "",
	}

	if err := createConfigMap(fakeKubeClient, ctx, NodeName, mSuccess); err != nil {
		t.Error(err)
	}
	if err := createConfigMap(fakeKubeClient, ctx, NodeNameFail, mFail); err != nil {
		t.Error(err)
	}
	statefile, err := state.New(filepath.Join(CsiDir, "state.json"))
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.CreateVolumeResponse
		wantErr bool
	}{
		{
			name: "empty args",
			fields: fields{
				cf: &Config{
					NodeID: NodeNameFail,
				},
				c: fakeKubeClient,
				s: statefile,
			},
			args: args{
				ctx: context.Background(),
				req: &csi.CreateVolumeRequest{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid volume capabilities",
			fields: fields{
				cf: &Config{
					NodeID: NodeNameFail,
				},
				c: fakeKubeClient,
				s: statefile,
			},
			args: args{
				ctx: context.Background(),
				req: &csi.CreateVolumeRequest{
					Name: rName.String(),
					Parameters: map[string]string{
						PVCName:      "pvcName",
						PVCNameSpace: "default",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "name is not specified",
			fields: fields{
				cf: &Config{
					NodeID: NodeNameFail,
				},
				c: fakeKubeClient,
				s: statefile,
			},
			args: args{
				ctx: context.Background(),
				req: &csi.CreateVolumeRequest{
					Name: "",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "access type is incorrect",
			fields: fields{
				cf: &Config{
					NodeID: NodeNameFail,
				},
				c: fakeKubeClient,
				s: statefile,
			},
			args: args{
				ctx: context.Background(),
				req: &csi.CreateVolumeRequest{
					Name: rName.String(),
					VolumeCapabilities: []*csi.VolumeCapability{
						{
							AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
							AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
						},
						{
							AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
							AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
						},
					},
					Parameters: map[string]string{
						PVCName:      "pvcName",
						PVCNameSpace: "default",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty device ID",
			fields: fields{
				cf: &Config{
					NodeID: NodeNameFail,
				},
				c: fakeKubeClient,
			},
			args: args{
				ctx: context.Background(),
				req: &csi.CreateVolumeRequest{
					Name: rName.String(),
					VolumeCapabilities: []*csi.VolumeCapability{
						{
							AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
							AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
						},
					},
					Parameters: map[string]string{
						PVCName:      "pvcName",
						PVCNameSpace: "default",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "exceed max allowed capacity",
			fields: fields{
				cf: &Config{
					NodeID:         NodeName,
					EnableTopology: true,
					StateDir:       CsiDir,
					MaxVolumeSize:  1024 * 1024 * 1024 * 1024,
				},
				c: fakeKubeClient,
				s: statefile,
			},
			args: args{
				ctx: context.Background(),
				req: &csi.CreateVolumeRequest{
					Name: rName.String(),
					VolumeCapabilities: []*csi.VolumeCapability{
						{
							AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
							AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
						},
					},
					Parameters: map[string]string{
						PVCName:      "pvcName",
						PVCNameSpace: "default",
						storageKind:  "",
					},
					CapacityRange: &csi.CapacityRange{RequiredBytes: int64(2 * 1024 * 1024 * 1024 * 1024)},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "fail to get mount point",
			fields: fields{
				cf: &Config{
					NodeID:         NodeName,
					EnableTopology: true,
					StateDir:       CsiDir,
					MaxVolumeSize:  1024 * 1024 * 1024 * 1024,
				},
				c: fakeKubeClient,
				s: statefile,
			},
			args: args{
				ctx: context.Background(),
				req: &csi.CreateVolumeRequest{
					Name: rName.String(),
					VolumeCapabilities: []*csi.VolumeCapability{
						{
							AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
							AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
						},
					},
					Parameters: map[string]string{
						PVCName:      "pvcName",
						PVCNameSpace: "default",
						storageKind:  "",
					},
					CapacityRange: &csi.CapacityRange{RequiredBytes: int64(2 * 1024 * 1024)},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := &hostPath{
				config:     *tt.fields.cf,
				coreClient: tt.fields.c,
				state:      tt.fields.s,
			}
			got, err := cs.CreateVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("controllerServer.CreateVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("controllerServer.CreateVolume() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerServer_DeleteVolume(t *testing.T) {
	type args struct {
		ctx context.Context
		req *csi.DeleteVolumeRequest
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

	// pvcPodSchedulerMap
	testfields := fields{
		cf: &Config{
			NodeID:               NodeName,
			EnableTopology:       true,
			StateDir:             CsiDir,
			MaxVolumeSize:        1024 * 1024 * 1024 * 1024,
			CheckVolumeLifecycle: true,
		},
		c: fakeKubeClient,
		s: statefile,
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.DeleteVolumeResponse
		wantErr bool
	}{
		{
			name:   "empty volume id",
			fields: testfields,
			args: args{
				ctx: context.Background(),
				req: &csi.DeleteVolumeRequest{
					VolumeId: "",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:   "unknown volume id",
			fields: testfields,
			args: args{
				ctx: context.Background(),
				req: &csi.DeleteVolumeRequest{
					VolumeId: "123456",
				},
			},
			want:    &csi.DeleteVolumeResponse{},
			wantErr: false,
		},
		{
			name:   "volume still used",
			fields: testfields,
			args: args{
				ctx: context.Background(),
				req: &csi.DeleteVolumeRequest{
					VolumeId: volumeId,
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := &hostPath{
				config:     *tt.fields.cf,
				coreClient: tt.fields.c,
				state:      tt.fields.s,
			}
			got, err := cs.DeleteVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("controllerServer.DeleteVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("controllerServer.DeleteVolume() = %v, want %v", got, tt.want)
			}
			if tt.name == "volume still used" {
				if err := cs.state.DeleteVolume(volumeId); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func Test_controllerServer_ControllerGetCapabilities(t *testing.T) {
	type args struct {
		ctx context.Context
		req *csi.ControllerGetCapabilitiesRequest
	}
	tests := []struct {
		name    string
		args    args
		want    *csi.ControllerGetCapabilitiesResponse
		wantErr bool
	}{
		{
			name: "success",
			args: args{
				ctx: context.Background(),
				req: &csi.ControllerGetCapabilitiesRequest{},
			},
			want: &csi.ControllerGetCapabilitiesResponse{
				Capabilities: []*csi.ControllerServiceCapability{
					{
						Type: &csi.ControllerServiceCapability_Rpc{
							Rpc: &csi.ControllerServiceCapability_RPC{
								Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
							},
						},
					},
					{
						Type: &csi.ControllerServiceCapability_Rpc{
							Rpc: &csi.ControllerServiceCapability_RPC{
								Type: csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
							},
						},
					},
					{
						Type: &csi.ControllerServiceCapability_Rpc{
							Rpc: &csi.ControllerServiceCapability_RPC{
								Type: csi.ControllerServiceCapability_RPC_VOLUME_CONDITION,
							},
						},
					},
					{
						Type: &csi.ControllerServiceCapability_Rpc{
							Rpc: &csi.ControllerServiceCapability_RPC{
								Type: csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
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
			got, err := cs.ControllerGetCapabilities(tt.args.ctx, tt.args.req)
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

func Test_hostPath_ValidateVolumeCapabilities(t *testing.T) {
	type fields struct {
		config     Config
		coreClient kubernetes.Interface
		// mutex      sync.Mutex
		state state.State
	}
	type args struct {
		ctx context.Context
		req *csi.ValidateVolumeCapabilitiesRequest
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
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.ValidateVolumeCapabilitiesResponse
		wantErr bool
	}{
		{
			name:   "valid volume id",
			fields: testfields,
			args: args{
				ctx: context.Background(),
				req: &csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: volumeId,
					VolumeCapabilities: []*csi.VolumeCapability{
						{
							AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
							AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
						},
						{
							AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
							AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
						},
					},
				},
			},
			want: &csi.ValidateVolumeCapabilitiesResponse{
				Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
					VolumeCapabilities: []*csi.VolumeCapability{
						{
							AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
							AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
						},
						{
							AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
							AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "invalid volume id",
			fields: testfields,
			args: args{
				ctx: context.Background(),
				req: &csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: "123456",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hp := &hostPath{
				config:     tt.fields.config,
				coreClient: tt.fields.coreClient,
				// mutex:      tt.fields.mutex,
				state: tt.fields.state,
			}
			got, err := hp.ValidateVolumeCapabilities(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("hostPath.ValidateVolumeCapabilities() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("hostPath.ValidateVolumeCapabilities() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hostPath_ListVolumes(t *testing.T) {
	type fields struct {
		config     Config
		coreClient kubernetes.Interface
		state      state.State
	}
	type args struct {
		ctx context.Context
		req *csi.ListVolumesRequest
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
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.ListVolumesResponse
		wantErr bool
	}{
		{
			name:   "valid volume request",
			fields: testfields,
			args: args{
				ctx: context.Background(),
				req: &csi.ListVolumesRequest{
					MaxEntries: 1,
				},
			},
			want: &csi.ListVolumesResponse{
				Entries: []*csi.ListVolumesResponse_Entry{
					{
						Volume: &csi.Volume{
							VolumeId:      volume.VolID,
							CapacityBytes: volume.VolSize,
						},
						Status: &csi.ListVolumesResponse_VolumeStatus{
							PublishedNodeIds: []string{""},
							VolumeCondition: &csi.VolumeCondition{
								Abnormal: true,
								Message:  "The source path of the volume doesn't exist",
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
			hp := &hostPath{
				config:     tt.fields.config,
				coreClient: tt.fields.coreClient,
				state:      tt.fields.state,
			}
			got, err := hp.ListVolumes(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("hostPath.ListVolumes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("hostPath.ListVolumes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hostPath_ControllerGetVolume(t *testing.T) {
	type fields struct {
		config     Config
		coreClient kubernetes.Interface
		state      state.State
	}
	type args struct {
		ctx context.Context
		req *csi.ControllerGetVolumeRequest
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
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.ControllerGetVolumeResponse
		wantErr bool
	}{
		{
			name:   "valid volume request",
			fields: testfields,
			args: args{
				ctx: context.Background(),
				req: &csi.ControllerGetVolumeRequest{
					VolumeId: volumeId,
				},
			},
			want: &csi.ControllerGetVolumeResponse{
				Volume: &csi.Volume{
					VolumeId:      volume.VolID,
					CapacityBytes: volume.VolSize,
				},
				Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
					PublishedNodeIds: []string{volume.NodeID},
					VolumeCondition: &csi.VolumeCondition{
						Abnormal: true,
						Message:  "The source path of the volume doesn't exist",
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "invalid volume id",
			fields: testfields,
			args: args{
				ctx: context.Background(),
				req: &csi.ControllerGetVolumeRequest{
					VolumeId: "123",
				},
			},
			want: &csi.ControllerGetVolumeResponse{
				Volume: &csi.Volume{
					VolumeId: "123",
				},
				Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
					VolumeCondition: &csi.VolumeCondition{
						Abnormal: true,
						Message:  "rpc error: code = NotFound desc = volume id 123 does not exist in the volumes list",
					},
				},
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
			got, err := hp.ControllerGetVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("hostPath.ControllerGetVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("hostPath.ControllerGetVolume() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalDeviceInUse(t *testing.T) {
	volumes := []state.Volume{
		{
			DeviceID: "device1",
		},
		{
			DeviceID: "device3",
		},
	}

	// Test case 1: inUse is true, device is already in use
	isUpdateCr, _, err := CalDeviceInUse("[\"device1\"]", "device1", true, volumes)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if isUpdateCr {
		t.Error("Expected isUpdateCr to be false")
	}

	// Test case 2: inUse is true, device is not in use
	isUpdateCr, new, err := CalDeviceInUse("[]", "device2", true, volumes)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !isUpdateCr {
		t.Error("Expected isUpdateCr to be true")
	}
	if new != "[\"device2\"]" {
		t.Error("Expected newDeviceInUse to be device2 ")
	}

	// Test case 3: inUse is false, device is not in use
	isUpdateCr, _, err = CalDeviceInUse("[]", "device2", false, volumes)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !isUpdateCr {
		t.Error("Expected isUpdateCr to be true")
	}

	// Test case 4
	isUpdateCr, new, err = CalDeviceInUse("[\"device2\"]", "device2", false, volumes)
	if err != nil {
		t.Error("Unexpected error, but got:", err)
	}
	if !isUpdateCr {
		t.Error("Expected isUpdateCr to be true")
	}
	if new != "[]" {
		t.Error("Expected newDeviceInUse to be []")
	}
}

func Test_hostPath_ControllerPublishVolume(t *testing.T) {
	hp := &hostPath{}
	_, err := hp.ControllerPublishVolume(context.Background(), &csi.ControllerPublishVolumeRequest{})
	assert.Equal(t, status.Error(codes.Unimplemented, ""), err)
}

func Test_hostPath_CreateSnapshot(t *testing.T) {
	hp := &hostPath{}
	_, err := hp.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{})
	assert.Equal(t, status.Error(codes.Unimplemented, ""), err)
}

func Test_hostPath_DeleteSnapshot(t *testing.T) {
	hp := &hostPath{}
	_, err := hp.DeleteSnapshot(context.Background(), &csi.DeleteSnapshotRequest{})
	assert.Equal(t, status.Error(codes.Unimplemented, ""), err)
}

func Test_hostPath_ControllerExpandVolume(t *testing.T) {
	hp := &hostPath{}
	_, err := hp.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{})
	assert.Equal(t, status.Error(codes.Unimplemented, ""), err)
}

func Test_hostPath_ListSnapshots(t *testing.T) {
	hp := &hostPath{}
	_, err := hp.ListSnapshots(context.Background(), &csi.ListSnapshotsRequest{})
	assert.Equal(t, status.Error(codes.Unimplemented, ""), err)
}

func Test_hostPath_TestControllerUnpublishVolume(t *testing.T) {
	hp := &hostPath{}
	_, err := hp.ControllerUnpublishVolume(context.Background(), &csi.ControllerUnpublishVolumeRequest{})
	assert.Equal(t, status.Error(codes.Unimplemented, ""), err)
}
