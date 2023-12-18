/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
)

func Test_calIopsByBW(t *testing.T) {
	type args struct {
		flag     string
		iopsData IOPSData
		bw       float64
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 int64
	}{
		{
			name:  "test convert data 1",
			args:  args{flag: "read", iopsData: IOPSData{RdReqBW: 4, RdTranBW: 20, RdTranRatio: 5, RdReqIops: 1024}, bw: 15 * utils.MiB},
			want:  fmt.Sprintf("%d", 3*int64(utils.MiB)),
			want1: 768,
		},
		{
			name:  "test convert data 2",
			args:  args{flag: "write", iopsData: IOPSData{WrReqBW: 4, WrTranBW: 8, WrTranRatio: 0, WrReqIops: 1024}, bw: 330 * utils.MiB},
			want:  "",
			want1: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := calIopsByBW(tt.args.flag, tt.args.iopsData, tt.args.bw)
			if got != tt.want {
				t.Errorf("calIopsByBW() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("calIopsByBW() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_parseReceiveMessageDisk(t *testing.T) {

	AppBwInfo := map[string]*pb.AppBandwidthInfo{
		"pod1": {
			BwInfo: []*pb.BandwidthInfo{
				{
					Id:     "disk1",
					IsRead: true,
					Bandwidths: map[string]string{
						"512b": "104857600",
						"1k":   "209715200",
						"4k":   "314572800",
						"8k":   "419430400",
						"16k":  "524288000",
						"32k":  "629145600",
					},
				},
				{
					Id:     "disk1",
					IsRead: false,
					Bandwidths: map[string]string{
						"512b": "734003200",
						"1k":   "838860800",
						"4k":   "943718400",
						"8k":   "1048576000",
						"16k":  "1153433600",
						"32k":  "1258291200",
					},
				},
				{
					Id:     "disk2",
					IsRead: true,
					Bandwidths: map[string]string{
						"512b": "734003200",
						"1k":   "838860800",
						"4k":   "943718400",
						"8k":   "1048576000",
						"16k":  "1153433600",
						"32k":  "1258291200",
					},
				},
				{
					Id:     "disk2",
					IsRead: false,
					Bandwidths: map[string]string{
						"512b": "2202009600",
						"1k":   "2516582400",
						"4k":   "2831155200",
						"8k":   "3145728000",
						"16k":  "3460300800",
						"32k":  "3774873600",
					},
				},
			},
		},
	}

	resp := &pb.AppsBandwidth{
		IoiType: 1,
		Payload: &pb.AppsBandwidth_DiskIoBw{
			DiskIoBw: &pb.AppDiskIOBandwidth{
				AppBwInfo: AppBwInfo,
			}},
	}

	DiskInfos := map[string]utils.DiskInfo{
		"disk1": {
			MajorMinor: "disk1",
			ReadRatio: map[string]float64{
				"512b": 0.1,
				"1k":   0.2,
				"4k":   0.3,
				"8k":   0.4,
				"16k":  0.5,
				"32k":  0.6,
			},
			WriteRatio: map[string]float64{
				"512b": 0.7,
				"1k":   0.8,
				"4k":   0.9,
				"8k":   1.0,
				"16k":  1.1,
				"32k":  1.2,
			},
		},
		"disk2": {
			MajorMinor: "disk2",
			ReadRatio: map[string]float64{
				"512b": 0.2,
				"1k":   0.4,
				"4k":   0.6,
				"8k":   0.8,
				"16k":  1.0,
				"32k":  1.2,
			},
			WriteRatio: map[string]float64{
				"512b": 0.3,
				"1k":   0.6,
				"4k":   0.9,
				"8k":   1.2,
				"16k":  1.5,
				"32k":  1.8,
			},
		},
	}

	expectedResult := &agent.PodData{
		PodEvents: map[string]agent.PodEventInfo{
			"disk1": {
				"pod1": {
					Actual: &utils.Workload{
						InBps:    1000,
						OutBps:   2100,
						TotalBps: 3100,
					},
					RawActualBs: &map[string]*utils.Workload{
						"512b": {
							InBps:    100,
							OutBps:   700,
							TotalBps: 800,
						},
						"1k": {
							InBps:    200,
							OutBps:   800,
							TotalBps: 1000,
						},
						"4k": {
							InBps:    300,
							OutBps:   900,
							TotalBps: 1200,
						},
						"8k": {
							InBps:    400,
							OutBps:   1000,
							TotalBps: 1400,
						},
						"16k": {
							InBps:    500,
							OutBps:   1100,
							TotalBps: 1600,
						},
						"32k": {
							InBps:    600,
							OutBps:   1200,
							TotalBps: 1800,
						},
					},
				},
			},
			"disk2": {
				"pod1": {
					Actual: &utils.Workload{
						InBps:    2100,
						OutBps:   4200,
						TotalBps: 6300,
					},
					RawActualBs: &map[string]*utils.Workload{
						"512b": {
							InBps:    700,
							OutBps:   2100,
							TotalBps: 2800,
						},
						"1k": {
							InBps:    800,
							OutBps:   2400,
							TotalBps: 3200,
						},
						"4k": {
							InBps:    900,
							OutBps:   2700,
							TotalBps: 3600,
						},
						"8k": {
							InBps:    1000,
							OutBps:   3000,
							TotalBps: 4000,
						},
						"16k": {
							InBps:    1100,
							OutBps:   3300,
							TotalBps: 4400,
						},
						"32k": {
							InBps:    1200,
							OutBps:   3600,
							TotalBps: 4800,
						},
					},
				},
			},
		},
	}

	result := parseReceiveMessageDisk(resp, DiskInfos)

	rDev1Pod1 := result.PodEvents["disk1"]["pod1"].RawActualBs
	rDev2Pod1 := result.PodEvents["disk2"]["pod1"].RawActualBs
	rDev1Pod2 := result.PodEvents["disk1"]["pod2"].RawActualBs
	rDev2Pod2 := result.PodEvents["disk2"]["pod2"].RawActualBs

	eDev1Pod1 := expectedResult.PodEvents["disk1"]["pod1"].RawActualBs
	eDev2Pod1 := expectedResult.PodEvents["disk2"]["pod1"].RawActualBs
	eDev1Pod2 := expectedResult.PodEvents["disk1"]["pod2"].RawActualBs
	eDev2Pod2 := expectedResult.PodEvents["disk2"]["pod2"].RawActualBs

	if !reflect.DeepEqual(rDev1Pod1, eDev1Pod1) {
		t.Errorf("parseReceiveMessageDisk() = %+v, want %+v", rDev1Pod1, eDev1Pod1)
	}
	if !reflect.DeepEqual(rDev2Pod1, eDev2Pod1) {
		t.Errorf("parseReceiveMessageDisk() = %+v, want %+v", rDev2Pod1, eDev2Pod1)
	}
	if !reflect.DeepEqual(rDev1Pod2, eDev1Pod2) {
		t.Errorf("parseReceiveMessageDisk() = %+v, want %+v", rDev1Pod2, eDev1Pod2)
	}
	if !reflect.DeepEqual(rDev2Pod2, eDev2Pod2) {
		t.Errorf("parseReceiveMessageDisk() = %+v, want %+v", rDev2Pod2, eDev2Pod2)
	}
}
func Test_deepCopyResult(t *testing.T) {
	previousResult := agent.PodData{
		T: 1,
		PodEvents: map[string]agent.PodEventInfo{
			"disk1": {
				"pod1": {
					Actual: &utils.Workload{
						InBps:    1000,
						OutBps:   2100,
						TotalBps: 3100,
					},
					RawActualBs: &map[string]*utils.Workload{
						"512b": {
							InBps:    100,
							OutBps:   700,
							TotalBps: 800,
						},
						"1k": {
							InBps:    200,
							OutBps:   800,
							TotalBps: 1000,
						},
					},
				},
			},
		},
	}

	averageResult := agent.PodData{
		T: 2,
		PodEvents: map[string]agent.PodEventInfo{
			"disk1": {
				"pod1": {
					Actual: &utils.Workload{
						InBps:    1500,
						OutBps:   2500,
						TotalBps: 4000,
					},
					RawActualBs: &map[string]*utils.Workload{
						"512b": {
							InBps:    150,
							OutBps:   750,
							TotalBps: 900,
						},
						"1k": {
							InBps:    250,
							OutBps:   850,
							TotalBps: 1100,
						},
					},
				},
			},
		},
	}

	expectedResult := agent.PodData{
		T: 2,
		PodEvents: map[string]agent.PodEventInfo{
			"disk1": {
				"pod1": {
					Actual: &utils.Workload{
						InBps:    1000,
						OutBps:   2100,
						TotalBps: 3100,
					},
					RawActualBs: &map[string]*utils.Workload{
						"512b": {
							InBps:    100,
							OutBps:   700,
							TotalBps: 800,
						},
						"1k": {
							InBps:    200,
							OutBps:   800,
							TotalBps: 1000,
						},
					},
				},
			},
		},
	}

	deepCopyResult(previousResult, averageResult)

	if !reflect.DeepEqual(averageResult, expectedResult) {
		t.Errorf("deepCopyResult() = %+v, want %+v", averageResult, expectedResult)
	}
}

func Test_compareResult(t *testing.T) {

	previousResult := agent.PodData{
		T: 1,
		PodEvents: map[string]agent.PodEventInfo{
			"disk1": {
				"pod1": {
					Actual: &utils.Workload{
						InBps:    1000,
						OutBps:   2100,
						TotalBps: 3100,
					},
					RawActualBs: &map[string]*utils.Workload{
						"512b": {
							InBps:    100,
							OutBps:   700,
							TotalBps: 800,
						},
						"1k": {
							InBps:    200,
							OutBps:   800,
							TotalBps: 1000,
						},
					},
				},
			},
		},
	}

	averageResult := agent.PodData{
		T: 2,
		PodEvents: map[string]agent.PodEventInfo{
			"disk1": {
				"pod1": {
					Actual: &utils.Workload{
						InBps:    1500,
						OutBps:   2500,
						TotalBps: 4000,
					},
					RawActualBs: &map[string]*utils.Workload{
						"512b": {
							InBps:    150,
							OutBps:   750,
							TotalBps: 900,
						},
						"1k": {
							InBps:    250,
							OutBps:   850,
							TotalBps: 1100,
						},
					},
				},
			},
		},
	}

	expectedResult := false

	result := compareResult(averageResult, previousResult)

	if result != expectedResult {
		t.Errorf("compareResult() = %v, want %v", result, expectedResult)
	}
}

func Test_IOISetLimitDisk(t *testing.T) {
	appId := "0.pod1"
	bw := &ComposeBWLimit{
		Id: "disk1",
		Wl: utils.Workload{
			InBps:   1000,
			OutBps:  2000,
			InIops:  3000,
			OutIops: 4000,
		},
	}
	w := &agent.WorkloadGroup{
		RawRequest: map[string]*utils.Workload{
			"pod1": {
				InBps:   500,
				OutBps:  1000,
				InIops:  1500,
				OutIops: 2000,
			},
		},
		RequestPods: map[string]*utils.Workload{
			"pod1": {
				InBps:   600,
				OutBps:  1200,
				InIops:  1800,
				OutIops: 2400,
			},
		},
	}

	expectedLimit := &ComposeBWLimit{
		Id: "disk1",
		Wl: utils.Workload{
			InBps:    1000,
			InIops:   3000,
			OutBps:   2000,
			OutIops:  4000,
			TotalBps: 0,
		},
	}

	IOISetLimitDisk(appId, bw, w)

	if !reflect.DeepEqual(bw, expectedLimit) {
		t.Errorf("IOISetLimitDisk() failed, expected: %+v, got: %+v", expectedLimit, bw)
	}
}

func TestIOIUnRegisterDisk(t *testing.T) {
	podUid := "test-pod-uid"

	// Create a channel to receive the requests sent to the clientHandlerChan
	requests := make(chan interface{}, 1)
	agent.ClientHandlerChan = requests

	// Call the function under test
	IOIUnRegisterDisk(podUid)

	// Verify that the correct request was sent to the clientHandlerChan
	select {
	case req := <-requests:
		unregisterReq, ok := req.(*pb.UnRegisterAppRequest)
		assert.True(t, ok, "unexpected request type")
		assert.Equal(t, podUid, unregisterReq.AppId, "unexpected AppId")
	default:
		assert.Fail(t, "no request sent to clientHandlerChan")
	}
}

func TestIOIGetServiceInfoDisk(t *testing.T) {
	type args struct {
		req string
	}
	tests := []struct {
		name string
		args args
		want *agent.AllProfile
	}{
		{
			name: "Test IOIGetServiceInfoDisk",
			args: args{
				req: `{"IoiType":1,"Payload":{"DiskIoBw":{"AppBwInfo":{"pod1":{"BwInfo":[{"Id":"disk1","IsRead":true,"Bandwidths":{"512b":"104857600","1k":"209715200","4k":"314572800","8k":"419430400","16k":"524288000","32k":"629145600"}},{"Id":"disk1","IsRead":false,"Bandwidths":{"512b":"734003200","1k":"838860800","4k":"943718400","8k":"1048576000","16k":"1153433600","32k":"1258291200"}},{"Id":"disk2","IsRead":true,"Bandwidths":{"512b":"734003200","1k":"838860800","4k":"943718400","8k":"1048576000","16k":"1153433600","32k":"1258291200"}},{"Id":"disk2","IsRead":false,"Bandwidths":{"512b":"2202009600","1k":"2516582400","4k":"2831155200","8k":"3145728000","16k":"3460300800","32k":"3774873600"}}]}}}}}`,
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IOIGetServiceInfoDisk(tt.args.req); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IOIGetServiceInfoDisk() = %v, want %v", got, tt.want)
			}
		})
	}
}
