/*
Copyright 2024 Intel Corporation

Licensed under the Apache License, Version 2.0 (the License);
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package iodriver

import (
	"context"
	"testing"

	"github.com/intel/cloud-resource-scheduling-and-isolation/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProcessProfile(t *testing.T) {
	ctx := context.Background()

	name := "test"
	d := &IODriver{
		excs:               fake.NewSimpleClientset(),
		cs:                 nil,
		nodeName:           name,
		diskInfos:          &DiskInfos{},
		observedGeneration: nil,
	}

	if err := d.ProcessProfile(ctx, GetDiskProfile()); err != nil {
		t.Fatalf("ProcessProfile failed: %v", err)
	}
	crName := GetCRName(name, NodeDiskDeviceCRSuffix)
	_, err := d.excs.DiskioV1alpha1().NodeDiskDevices(CRNameSpace).Get(ctx, crName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		t.Errorf("The NodeDiskDevice CR (%s) has not been created.", crName)
	}
	assert.NotNil(t, d.diskInfos.Info)
	info, ok := d.diskInfos.Info[FakeDeviceID]
	if !ok {
		t.Error("disk info is not saved")
	}
	assert.Equal(t, info, GetFakeDevice())
}
