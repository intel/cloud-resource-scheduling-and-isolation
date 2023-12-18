/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	ioiresource "sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
)

func TestResource_AdmitPod(t *testing.T) {
	type fields struct {
		nodeName string
		info     *NodeInfo
		ch       ioiresource.CacheHandle
		// RWMutex  sync.RWMutex
	}
	type args struct {
		pod *v1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Admit new pod",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					RDTClassStatus: map[string]float64{
						"class1": 100,
						"class2": 80,
						"class3": 60,
					},
				},
				ch: &Handle{
					HandleBase: ioiresource.HandleBase{
						EC: fakeResourceCache(make(map[string]float64)),
					},
				},
			},
			args: args{
				pod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod1",
						Annotations: map[string]string{
							utils.RDTConfigAnno: "class1",
						},
					},
				},
			},
			want: &RequestInfo{
				RDTClass: "class1",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &Resource{
				nodeName: tt.fields.nodeName,
				info:     tt.fields.info,
				ch:       tt.fields.ch,
				// RWMutex:  tt.fields.RWMutex,
			}
			got, err := ps.AdmitPod(tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resource.AdmitPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Resource.AdmitPod() = %v, want %v", got, tt.want)
			}
		})
	}
}
