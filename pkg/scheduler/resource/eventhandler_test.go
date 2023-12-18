/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package resource

import (
	"context"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/runtime"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned/fake"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	common "sigs.k8s.io/IOIsolation/pkg/scheduler/util"
)

func TestComposeAllocableBW(t *testing.T) {
	type fields struct {
		nio *v1.NodeStaticIOInfo
		typ string
	}
	tests := []struct {
		name string
		args *fields
		want map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth
	}{
		{
			name: "CR NodeStaticIOInfo is nil",
			args: &fields{
				nio: nil,
				typ: "",
			},
			want: nil,
		},
		{
			name: "ResourceConfig is nil",
			args: &fields{
				nio: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{},
				},
			},
			want: nil,
		},
		{
			name: "ResourceConfig is empty",
			args: &fields{
				nio: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						ResourceConfig: map[string]v1.ResourceConfigSpec{},
					},
				},
			},
			want: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{},
		},
		{
			name: "Compose unknown IO types",
			args: &fields{
				nio: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						ResourceConfig: map[string]v1.ResourceConfigSpec{
							string(v1.BlockIO): {
								Devices: map[string]v1.Device{
									"dev1": {},
									"dev2": {},
								},
							},
							string(v1.NetworkIO): {
								Devices: map[string]v1.Device{
									"dev1": {},
									"dev2": {},
								},
							},
						},
					},
				},
				typ: "abc",
			},
			want: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{},
		},
		{
			name: "Compose one IO types",
			args: &fields{
				nio: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						ResourceConfig: map[string]v1.ResourceConfigSpec{
							string(v1.BlockIO): {
								Devices: map[string]v1.Device{
									"dev1": {},
									"dev2": {},
								},
							},
							string(v1.NetworkIO): {
								Devices: map[string]v1.Device{
									"dev1": {},
									"dev2": {},
								},
							},
						},
					},
				},
				typ: "BlockIO",
			},
			want: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
				v1.BlockIO: {
					DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
						"dev1": {
							IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
								v1.WorkloadGA: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
								v1.WorkloadBE: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
							},
						},
						"dev2": {
							IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
								v1.WorkloadGA: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
								v1.WorkloadBE: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Compose all IO types",
			args: &fields{
				nio: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						ResourceConfig: map[string]v1.ResourceConfigSpec{
							string(v1.BlockIO): {
								Devices: map[string]v1.Device{
									"dev1": {},
									"dev2": {},
								},
							},
							string(v1.NetworkIO): {
								Devices: map[string]v1.Device{
									"dev1": {},
									"dev2": {},
								},
							},
						},
					},
				},
				typ: "",
			},
			want: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
				v1.BlockIO: {
					DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
						"dev1": {
							IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
								v1.WorkloadGA: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
								v1.WorkloadBE: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
							},
						},
						"dev2": {
							IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
								v1.WorkloadGA: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
								v1.WorkloadBE: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
							},
						},
					},
				},
				v1.NetworkIO: {
					DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
						"dev1": {
							IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
								v1.WorkloadGA: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
								v1.WorkloadBE: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
							},
						},
						"dev2": {
							IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
								v1.WorkloadGA: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
								v1.WorkloadBE: {
									Total:    -1,
									In:       -1,
									Out:      -1,
									Pressure: -1,
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := composeAllocableBW(tt.args.nio, tt.args.typ); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("composeAllocableBW got %v, want %v", got, tt.want)
			}
		})
	}
}
func Test_composeDeviceAllocableBW(t *testing.T) {
	type args struct {
		adds map[string]v1.Device
		dels map[string]v1.Device
	}
	tests := []struct {
		name  string
		args  args
		want  map[string]v1.IOAllocatableBandwidth
		want1 map[string]v1.IOAllocatableBandwidth
	}{
		{
			name: "Compose Device Allocable BW",
			args: args{
				adds: map[string]v1.Device{
					"dev1": {},
				},
				dels: map[string]v1.Device{
					"dev2": {},
				},
			},
			want: map[string]v1.IOAllocatableBandwidth{
				"dev1": {
					IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
						v1.WorkloadGA: {
							Total:    -1,
							In:       -1,
							Out:      -1,
							Pressure: -1,
						},
						v1.WorkloadBE: {
							Total:    -1,
							In:       -1,
							Out:      -1,
							Pressure: -1,
						},
					},
				},
			},
			want1: map[string]v1.IOAllocatableBandwidth{
				"dev2": {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := composeDeviceAllocableBW(tt.args.adds, tt.args.dels)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("composeDeviceAllocableBW() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("composeDeviceAllocableBW() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestIOEventHandler_AddAdminConfigData(t *testing.T) {
	type fields struct {
		cache      map[string]Handle
		coreClient kubernetes.Interface
		vClient    versioned.Interface
		f          informers.SharedInformerFactory
	}
	type args struct {
		obj interface{}
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantinfo  string
		wantdebug string
	}{
		{
			name: "Info and Debug are unset",
			args: args{
				obj: &corev1.ConfigMap{
					Data: map[string]string{
						"loglevel": "",
					},
				},
			},
			wantinfo:  "3",
			wantdebug: "4",
		},
		{
			name: "Debug set to true",
			args: args{
				obj: &corev1.ConfigMap{
					Data: map[string]string{
						"loglevel": "level=debug\n",
					},
				},
			},
			wantinfo:  "1",
			wantdebug: "2",
		},
	}
	for _, tt := range tests {
		utils.INF = 3
		utils.DBG = 4
		t.Run(tt.name, func(t *testing.T) {
			eh := &IOEventHandler{
				cache:      tt.fields.cache,
				coreClient: tt.fields.coreClient,
				vClient:    tt.fields.vClient,
				f:          tt.fields.f,
			}
			eh.AddAdminConfigData(tt.args.obj)
			if !reflect.DeepEqual(utils.INF.String(), tt.wantinfo) {
				t.Errorf("utils.INF %v, want %v", utils.INF, tt.wantinfo)
			}
			if !reflect.DeepEqual(utils.DBG.String(), tt.wantdebug) {
				t.Errorf("utils.DBG %v, want %v", utils.DBG, tt.wantdebug)
			}
		})
	}
}

func TestIOEventHandler_UpdateAdminConfigData(t *testing.T) {
	type fields struct {
		cache      map[string]Handle
		coreClient kubernetes.Interface
		vClient    versioned.Interface
		f          informers.SharedInformerFactory
	}
	type args struct {
		oldObj interface{}
		newObj interface{}
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantinfo  string
		wantdebug string
	}{
		{
			name: "Info and Debug are unset",
			args: args{
				newObj: &corev1.ConfigMap{
					Data: map[string]string{
						"loglevel": "",
					},
				},
			},
			wantinfo:  "3",
			wantdebug: "4",
		},
		{
			name: "Info set to true",
			args: args{
				newObj: &corev1.ConfigMap{
					Data: map[string]string{
						"loglevel": "level=info\n",
					},
				},
			},
			wantinfo:  "1",
			wantdebug: "4",
		},
	}
	for _, tt := range tests {
		utils.INF = 3
		utils.DBG = 4
		t.Run(tt.name, func(t *testing.T) {
			eh := &IOEventHandler{
				cache:      tt.fields.cache,
				coreClient: tt.fields.coreClient,
				vClient:    tt.fields.vClient,
				f:          tt.fields.f,
			}
			eh.UpdateAdminConfigData(tt.args.oldObj, tt.args.newObj)
			if !reflect.DeepEqual(utils.INF.String(), tt.wantinfo) {
				t.Errorf("utils.INF %v, want %v", utils.INF, tt.wantinfo)
			}
			if !reflect.DeepEqual(utils.DBG.String(), tt.wantdebug) {
				t.Errorf("utils.DBG %v, want %v", utils.DBG, tt.wantdebug)
			}
		})
	}
}

func TestIOEventHandler_AddNodeStaticIOInfo(t *testing.T) {
	type fields struct {
		cache      map[string]Handle
		coreClient kubernetes.Interface
		vClient    versioned.Interface
		f          informers.SharedInformerFactory
	}
	type args struct {
		obj interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "handle add node static IO info",
			fields: fields{
				cache: map[string]Handle{
					"NetworkIO": nil,
				},
				coreClient: fakekubeclientset.NewSimpleClientset(),
				vClient:    fake.NewSimpleClientset(),
			},
			args: args{
				obj: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						NodeName:       "node1",
						ResourceConfig: map[string]v1.ResourceConfigSpec{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		IoiContext = &ResourceIOContext{
			Reservedpod:    make(map[string]*pb.PodList),
			lastUpdatedGen: make(map[string]int64),
		}
		t.Run(tt.name, func(t *testing.T) {
			eh := &IOEventHandler{
				cache:      tt.fields.cache,
				coreClient: tt.fields.coreClient,
				vClient:    tt.fields.vClient,
				f:          tt.fields.f,
			}
			eh.AddNodeStaticIOInfo(tt.args.obj)
		})
	}
}

var scName string = "sc1"

func TestIOEventHandler_getBoundDeviceId(t *testing.T) {
	type fields struct {
		cache      map[string]Handle
		coreClient kubernetes.Interface
		vClient    versioned.Interface
		f          informers.SharedInformerFactory
	}
	type args struct {
		pvc  *corev1.PersistentVolumeClaim
		node string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		want1   int64
		wantErr bool
	}{
		{
			name: "pvc is nil",
			args: args{
				pvc:  nil,
				node: "node1",
			},
			want:    "",
			want1:   -1,
			wantErr: true,
		},
		{
			name: "get bounded info",
			args: args{
				pvc: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc1",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &scName,
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								"storage": resource.MustParse("10"),
							},
						},
					},
				},
				node: "node1",
			},
			want:    "dev1",
			want1:   10,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		IoiContext = &ResourceIOContext{
			ClaimDevMap: map[string]*StorageInfo{
				"pvc1.default": {},
			},
		}
		t.Run(tt.name, func(t *testing.T) {
			eh := &IOEventHandler{
				cache:      tt.fields.cache,
				coreClient: tt.fields.coreClient,
				vClient:    tt.fields.vClient,
				f:          tt.fields.f,
			}
			patches := gomonkey.ApplyFunc(common.GetVolumeDeviceMap, func(client kubernetes.Interface, nodename string) (*corev1.ConfigMap, error) {
				return &corev1.ConfigMap{
					Data: map[string]string{
						"pvc1.default": "dev1:/tmp/dev1",
					},
				}, nil
			})
			defer patches.Reset()
			got, got1, err := eh.getBoundDeviceId(tt.args.pvc, tt.args.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("IOEventHandler.getBoundDeviceId() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IOEventHandler.getBoundDeviceId() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("IOEventHandler.getBoundDeviceId() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestIOEventHandler_UpdatePVC(t *testing.T) {
	type fields struct {
		cache      map[string]Handle
		coreClient kubernetes.Interface
		vClient    versioned.Interface
		f          informers.SharedInformerFactory
	}
	type args struct {
		oldObj interface{}
		newObj interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "update pvc, unbounded to bounded",
			args: args{
				oldObj: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc1",
						Namespace: "default",
						Annotations: map[string]string{
							"volume.kubernetes.io/selected-node":       "node1",
							"volume.kubernetes.io/storage-provisioner": "localstorage.csi.k8s.io",
						},
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimPending,
					},
				},
				newObj: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc1",
						Namespace: "default",
						Annotations: map[string]string{
							"volume.kubernetes.io/selected-node":       "node1",
							"volume.kubernetes.io/storage-provisioner": "localstorage.csi.k8s.io",
						},
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimBound,
					},
				},
			},
		},
		{
			name: "old obj is nil",
			args: args{
				oldObj: nil,
				newObj: nil,
			},
		},
		{
			name: "new obj is nil",
			args: args{
				newObj: nil,
				oldObj: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc1",
						Namespace: "default",
						Annotations: map[string]string{
							"volume.kubernetes.io/selected-node":       "node1",
							"volume.kubernetes.io/storage-provisioner": "localstorage.csi.k8s.io",
						},
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimBound,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := &IOEventHandler{
				cache:      tt.fields.cache,
				coreClient: tt.fields.coreClient,
				vClient:    tt.fields.vClient,
				f:          tt.fields.f,
			}

			patches := gomonkey.ApplyPrivateMethod(reflect.TypeOf(eh), "getBoundDeviceId", func(_ *IOEventHandler, pvc *corev1.PersistentVolumeClaim, node string) (string, int64, error) {
				return "dev1", 10, nil
			})

			defer patches.Reset()
			eh.UpdatePVC(tt.args.oldObj, tt.args.newObj)
		})
	}
}

func TestIOEventHandler_DeletePVC(t *testing.T) {
	type fields struct {
		cache      map[string]Handle
		coreClient kubernetes.Interface
		vClient    versioned.Interface
		f          informers.SharedInformerFactory
	}
	type args struct {
		obj interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "pvc is nil",
			args: args{
				obj: nil,
			},
		},
		{
			name: "delete pvc",
			args: args{
				obj: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc1",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &scName,
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								"storage": resource.MustParse("10"),
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		IoiContext = &ResourceIOContext{
			ClaimDevMap: make(map[string]*StorageInfo),
		}
		t.Run(tt.name, func(t *testing.T) {
			eh := &IOEventHandler{
				cache:      tt.fields.cache,
				coreClient: tt.fields.coreClient,
				vClient:    tt.fields.vClient,
				f:          tt.fields.f,
			}
			eh.DeletePVC(tt.args.obj)
		})
	}
}

func TestNewIOEventHandler(t *testing.T) {
	type args struct {
		cache map[string]Handle
		h     framework.Handle
	}
	fw, err := runtime.NewFramework(context.Background(),
		nil,
		nil,
		runtime.WithInformerFactory(nil),
		runtime.WithClientSet(nil),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args args
		want *IOEventHandler
	}{
		{
			name: "New IOEventHandler",
			args: args{
				cache: map[string]Handle{},
				h:     fw,
			},
			want: &IOEventHandler{
				vClient:    nil,
				cache:      map[string]Handle{},
				coreClient: nil,
				f:          nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			IoiContext = &ResourceIOContext{
				VClient: nil,
			}
			if got := NewIOEventHandler(tt.args.cache, tt.args.h); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewIOEventHandler() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIOEventHandler_UpdateNodeIOStatus(t *testing.T) {
	client := fakekubeclientset.NewSimpleClientset()
	vclient := fake.NewSimpleClientset()
	nn := "testNode"

	type fields struct {
		coreClient   kubernetes.Interface
		vClient      versioned.Interface
		reservedPods map[string]*pb.PodList
	}
	type args struct {
		oldObj interface{}
		newObj interface{}
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantChange bool
	}{
		{
			name: "status is nil",
			args: args{
				oldObj: &v1.NodeIOStatus{
					TypeMeta: metav1.TypeMeta{
						APIVersion: utils.APIVersion,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: utils.CrNamespace,
						Name:      nn + utils.NodeStatusInfoSuffix,
					},
					Spec: v1.NodeIOStatusSpec{
						NodeName: nn,
					},
				},
				newObj: &v1.NodeIOStatus{
					TypeMeta: metav1.TypeMeta{
						APIVersion: utils.APIVersion,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: utils.CrNamespace,
						Name:      nn + utils.NodeStatusInfoSuffix,
					},
					Spec: v1.NodeIOStatusSpec{
						NodeName: nn,
					},
				},
			},
			fields: fields{
				coreClient: client,
				vClient:    vclient,
				reservedPods: map[string]*pb.PodList{
					nn: nil,
				},
			},
			wantChange: false,
		},
		{
			name: "failed to get reserved pods",
			args: args{
				oldObj: &v1.NodeIOStatus{
					TypeMeta: metav1.TypeMeta{
						APIVersion: utils.APIVersion,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: utils.CrNamespace,
						Name:      nn + utils.NodeStatusInfoSuffix,
					},
					Spec: v1.NodeIOStatusSpec{
						NodeName: nn,
					},
					Status: v1.NodeIOStatusStatus{},
				},
				newObj: &v1.NodeIOStatus{
					TypeMeta: metav1.TypeMeta{
						APIVersion: utils.APIVersion,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: utils.CrNamespace,
						Name:      nn + utils.NodeStatusInfoSuffix,
					},
					Spec: v1.NodeIOStatusSpec{
						NodeName: nn,
					},
					Status: v1.NodeIOStatusStatus{},
				},
			},
			fields: fields{
				coreClient:   client,
				vClient:      vclient,
				reservedPods: map[string]*pb.PodList{},
			},
			wantChange: false,
		},
		{
			name: "success",
			args: args{
				oldObj: &v1.NodeIOStatus{
					TypeMeta: metav1.TypeMeta{
						APIVersion: utils.APIVersion,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: utils.CrNamespace,
						Name:      nn + utils.NodeStatusInfoSuffix,
					},
					Spec: v1.NodeIOStatusSpec{
						NodeName: nn,
					},
					Status: v1.NodeIOStatusStatus{
						ObservedGeneration:   0,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{},
					},
				},
				newObj: &v1.NodeIOStatus{
					TypeMeta: metav1.TypeMeta{
						APIVersion: utils.APIVersion,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: utils.CrNamespace,
						Name:      nn + utils.NodeStatusInfoSuffix,
					},
					Spec: v1.NodeIOStatusSpec{
						NodeName: nn,
					},
					Status: v1.NodeIOStatusStatus{
						ObservedGeneration:   1,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{},
					},
				},
			},
			fields: fields{
				coreClient: client,
				vClient:    vclient,
				reservedPods: map[string]*pb.PodList{
					nn: {
						Generation: 1,
					},
				},
			},
			wantChange: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			IoiContext = &ResourceIOContext{
				Reservedpod: tt.fields.reservedPods,
			}
			var status *v1.NodeIOStatusStatus
			fh := &FakeHandle{
				UpdateNodeIOStatusFunc: func(n string, nodeIoBw *v1.NodeIOStatusStatus) error {
					status = nodeIoBw
					return nil
				},
			}
			fh.Run(&FakeCache{}, nil, tt.fields.coreClient)
			eh := &IOEventHandler{
				cache:      map[string]Handle{"Fake": fh},
				coreClient: tt.fields.coreClient,
				vClient:    tt.fields.vClient,
			}
			eh.UpdateNodeIOStatus(tt.args.oldObj, tt.args.newObj)
			if tt.wantChange {
				got := *status
				want := tt.args.newObj.(*v1.NodeIOStatus).Status
				t.Logf("got: %v, want: %v", got, want)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("UpdateNodeIOStatus() failed, want %v, got %v", want, got)
				}
			}
		})
	}
}

func TestIOEventHandler_DeleteNode(t *testing.T) {
	client := fakekubeclientset.NewSimpleClientset()
	vclient := fake.NewSimpleClientset()
	nn := "testNode"
	type fields struct {
		coreClient   kubernetes.Interface
		vClient      versioned.Interface
		reservedPods map[string]*pb.PodList
	}
	type args struct {
		obj interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				coreClient:   client,
				vClient:      vclient,
				reservedPods: map[string]*pb.PodList{},
			},
			args: args{
				obj: st.MakeNode().Name(nn).Obj(),
			},
			wantErr: false,
		},
		{
			name: "success and remove node in reserved pod",
			fields: fields{
				coreClient: client,
				vClient:    vclient,
				reservedPods: map[string]*pb.PodList{
					nn: nil,
				},
			},
			args: args{
				obj: st.MakeNode().Name(nn).Obj(),
			},
			wantErr: false,
		},
		{
			name: "obj cannot be converted to *v1.Node",
			fields: fields{
				coreClient:   client,
				vClient:      vclient,
				reservedPods: nil,
			},
			args: args{
				obj: corev1.Pod{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			IoiContext = &ResourceIOContext{
				Reservedpod: tt.fields.reservedPods,
			}
			var gotNodeName string
			fh := &FakeHandle{
				DeleteCacheNodeInfoFunc: func(nn string) error {
					gotNodeName = nn
					return nil
				},
			}
			fh.Run(&FakeCache{}, nil, tt.fields.coreClient)
			eh := &IOEventHandler{
				cache:      map[string]Handle{"Fake": fh},
				coreClient: tt.fields.coreClient,
				vClient:    tt.fields.vClient,
			}
			eh.DeleteNode(tt.args.obj)
			if !tt.wantErr {
				node := tt.args.obj.(*corev1.Node).Name
				if gotNodeName != node {
					t.Errorf("DeleteNode() failed in DeleteCacheNodeInfo, want %v, got %v", nn, gotNodeName)
					return
				}
				if _, err := IoiContext.GetReservedPods(node); err == nil {
					t.Errorf("DeleteNode() failed in IoiContext, node %s exists", nn)
					return
				}
				_, err := common.GetVolumeDeviceMap(eh.coreClient, node)
				if err == nil {
					t.Errorf("DeleteNode() failed in VolumeDeviceMap, node %s exists", nn)
				}
			}
		})
	}
}

func TestIOEventHandler_UpdateNode(t *testing.T) {
	client := fakekubeclientset.NewSimpleClientset()
	vclient := fake.NewSimpleClientset()
	nn := "testNode"
	type fields struct {
		coreClient kubernetes.Interface
		vClient    versioned.Interface
	}
	type args struct {
		oldObj interface{}
		newObj interface{}
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		isDeleteCache bool
	}{
		{
			name: "success",
			fields: fields{
				coreClient: client,
				vClient:    vclient,
			},
			args: args{
				newObj: st.MakeNode().Name(nn).Obj(),
				oldObj: st.MakeNode().Name(nn).Label("ioisolation", "net").Obj(),
			},
			isDeleteCache: true,
		},
		{
			name: "neither old and new have label",
			fields: fields{
				coreClient: client,
				vClient:    vclient,
			},
			args: args{
				newObj: st.MakeNode().Name(nn).Obj(),
				oldObj: st.MakeNode().Name(nn).Obj(),
			},
			isDeleteCache: false,
		},
		{
			name: "newObj cannot convert to *v1.Node ",
			fields: fields{
				coreClient: client,
				vClient:    vclient,
			},
			args: args{
				newObj: corev1.Pod{},
				oldObj: st.MakeNode().Name(nn).Obj(),
			},
			isDeleteCache: false,
		},
		{
			name: "oldObj cannot convert to *v1.Node ",
			fields: fields{
				coreClient: client,
				vClient:    vclient,
			},
			args: args{
				oldObj: corev1.Pod{},
				newObj: st.MakeNode().Name(nn).Obj(),
			},
			isDeleteCache: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotNodeName string
			fh := &FakeHandle{
				DeleteCacheNodeInfoFunc: func(nn string) error {
					gotNodeName = nn
					return nil
				},
			}
			fh.Run(&FakeCache{}, nil, tt.fields.coreClient)
			eh := &IOEventHandler{
				cache:      map[string]Handle{"Fake": fh},
				coreClient: tt.fields.coreClient,
				vClient:    tt.fields.vClient,
			}
			eh.UpdateNode(tt.args.oldObj, tt.args.newObj)
			if tt.isDeleteCache {
				node := tt.args.newObj.(*corev1.Node).Name
				if gotNodeName != node {
					t.Errorf("UpdateNode() failed in DeleteCacheNodeInfo, want %v, got %v", nn, gotNodeName)
					return
				}
			}
		})
	}
}

func TestIOEventHandler_DeletePod(t *testing.T) {
	client := fakekubeclientset.NewSimpleClientset()
	vclient := fake.NewSimpleClientset()
	nn := "testNode"
	rl := workqueue.DefaultControllerRateLimiter()
	type fields struct {
		coreClient   kubernetes.Interface
		vClient      versioned.Interface
		reservedPods map[string]*pb.PodList
	}
	type args struct {
		obj interface{}
	}
	node := st.MakePod().Name("pod1").Node(nn).UID("12345678").Namespace("default").Obj()
	node.Annotations = map[string]string{
		"ioi.intel.com/allocated-io": "",
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "obj cannot be convertd to *v1.Pod",
			fields: fields{
				coreClient:   client,
				vClient:      vclient,
				reservedPods: map[string]*pb.PodList{},
			},
			args: args{
				obj: corev1.Node{},
			},
			wantErr: true,
		},
		{
			name: "success",
			fields: fields{
				coreClient: client,
				vClient:    vclient,
				reservedPods: map[string]*pb.PodList{
					nn: {
						PodList: map[string]*pb.PodRequest{
							"12345678": {
								PodName:      "pod1",
								PodNamespace: "default",
								Request: []*pb.IOResourceRequest{
									{
										IoType:       pb.IOType_DiskIO,
										WorkloadType: utils.BeIndex,
										DevRequest: &pb.DeviceRequirement{
											DevName: "dev1",
											Request: &pb.Quantity{
												Inbps:  100,
												Outbps: 100,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				obj: node,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			IoiContext = &ResourceIOContext{
				Reservedpod: tt.fields.reservedPods,
				queue:       workqueue.NewNamedRateLimitingQueue(rl, "test"),
			}
			// go IoiContext.RunWorkerQueue(ctx)
			var gotRemovePod *corev1.Pod
			fh := &FakeHandle{}
			fh.Run(&FakeCache{
				GetCacheFunc: func(string) ExtendedResource {
					return &FakeResource{
						RemovePodFunc: func(pod *corev1.Pod, _ kubernetes.Interface) error {
							gotRemovePod = pod
							return nil
						},
					}
				},
			}, nil, tt.fields.coreClient)
			eh := &IOEventHandler{
				cache:      map[string]Handle{"Fake": fh},
				coreClient: tt.fields.coreClient,
				vClient:    tt.fields.vClient,
				f:          nil,
			}
			eh.DeletePod(tt.args.obj)
			if !tt.wantErr {
				if !reflect.DeepEqual(tt.args.obj, gotRemovePod) {
					t.Errorf("DeletePod() failed, gotRemovePod is %v, pod to delete is %v", gotRemovePod, tt.args.obj)
				}
				v, err := IoiContext.GetReservedPods(nn)
				if err != nil {
					t.Fatal(err)
				}
				if len(v.PodList) != 0 {
					t.Errorf("DeletePod() failed, pod %v is not deleted in Podlist", tt.args.obj.(*corev1.Pod).Name)
				}
			}
		})
	}
}

func TestIOEventHandler_DeleteNodeStaticIOInfo(t *testing.T) {
	client := fakekubeclientset.NewSimpleClientset()
	vclient := fake.NewSimpleClientset()
	nn := "testNode"
	type fields struct {
		coreClient   kubernetes.Interface
		vClient      versioned.Interface
		reservedPods map[string]*pb.PodList
	}
	type args struct {
		obj interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				coreClient:   client,
				vClient:      vclient,
				reservedPods: map[string]*pb.PodList{},
			},
			args: args{
				obj: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						NodeName:       "node1",
						ResourceConfig: map[string]v1.ResourceConfigSpec{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "success and remove node in reserved pod",
			fields: fields{
				coreClient: client,
				vClient:    vclient,
				reservedPods: map[string]*pb.PodList{
					nn: nil,
				},
			},
			args: args{
				obj: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						NodeName:       nn,
						ResourceConfig: map[string]v1.ResourceConfigSpec{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "obj cannot be converted to *v1.NodeStaticIOInfo",
			fields: fields{
				coreClient:   client,
				vClient:      vclient,
				reservedPods: nil,
			},
			args: args{
				obj: corev1.Pod{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			IoiContext = &ResourceIOContext{
				Reservedpod: tt.fields.reservedPods,
			}
			var gotNodeName string
			fh := &FakeHandle{
				DeleteCacheNodeInfoFunc: func(nn string) error {
					gotNodeName = nn
					return nil
				},
			}
			fh.Run(&FakeCache{}, nil, tt.fields.coreClient)
			eh := &IOEventHandler{
				cache:      map[string]Handle{"Fake": fh},
				coreClient: tt.fields.coreClient,
				vClient:    tt.fields.vClient,
			}
			eh.DeleteNodeStaticIOInfo(tt.args.obj)
			if !tt.wantErr {
				node := tt.args.obj.(*v1.NodeStaticIOInfo).Spec.NodeName
				if gotNodeName != node {
					t.Errorf("DeleteNodeStaticIOInfo() failed in DeleteCacheNodeInfo, want %v, got %v", nn, gotNodeName)
					return
				}
				if _, err := IoiContext.GetReservedPods(node); err == nil {
					t.Errorf("DeleteNodeStaticIOInfo() failed in IoiContext, node %s exists", nn)
					return
				}
			}
		})
	}
}
