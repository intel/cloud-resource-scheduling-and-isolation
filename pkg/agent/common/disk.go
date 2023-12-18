/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
)

const (
	ioiNum      = 6
	AverageTime = 30
	IOMAX       = 10000
)

var MinPodDiskInBw float64 = 5.0
var MinPodDiskOutBw float64 = 5.0

// 60M = c*x*cr*4K + b*x*br*8K  b,c is iops ratio, x is total iops, cr,be is 4k,8k transer ratio
func calIopsByBW(flag string, iopsdata IOPSData, bw float64) (string, int64) {

	var reqBw, tranBwRatio, reqIops float64

	if flag == "read" {
		reqBw = iopsdata.RdReqBW
		tranBwRatio = iopsdata.RdTranRatio
		reqIops = float64(iopsdata.RdReqIops)
	}

	if flag == "write" {
		reqBw = iopsdata.WrReqBW
		tranBwRatio = iopsdata.WrTranRatio
		reqIops = float64(iopsdata.WrReqIops)
	}

	if reqBw == 0 || tranBwRatio == 0 {
		klog.Errorf("Error: data is not right reqBw: %f, tranBwRatio: %f", reqBw, tranBwRatio)
		return "", 0
	}

	limit_bw := bw / tranBwRatio
	reqBw = reqBw * utils.MiB

	limitBw := strconv.Itoa(int(limit_bw))
	limitIops := int64(reqIops * limit_bw / reqBw)

	return limitBw, limitIops
}

func IOISetLimitDisk(appId string, bw *ComposeBWLimit, w *agent.WorkloadGroup) {
	var bws []*pb.BWLimit
	var lmt pb.BWLimit

	lmt.Id = bw.Id
	podIds := strings.Split(appId, ".")
	podId := podIds[1]
	max := int64(IOMAX * utils.MiB)

	if bw.Wl.InBps == LimitMax {
		lmt.In = fmt.Sprintf("%d", max)
		lmt.InIops = max / 4096
	} else if bw.Wl.InBps == LimitKeepValue {
		lmt.In = ""
		lmt.InIops = 0
	} else {
		if bw.Wl.InBps < MinPodDiskInBw {
			bw.Wl.InBps = MinPodDiskInBw
		}

		if w.WorkloadType == utils.BeIndex {
			lmt.In = fmt.Sprintf("%d", int64(bw.Wl.InBps*utils.MiB))
			lmt.InIops = int64(bw.Wl.InBps*utils.MiB) / 32768
		} else {
			total_bw := bw.Wl.InBps * utils.MiB
			var iopsdata IOPSData
			flag := "read"

			iopsdata.RdReqBW = w.RawRequest[podId].InBps
			iopsdata.RdTranBW = w.RequestPods[podId].InBps
			iopsdata.RdTranRatio = iopsdata.RdTranBW / iopsdata.RdReqBW
			iopsdata.RdReqIops = w.RawRequest[podId].InIops

			klog.V(utils.DBG).Infof("SetAppLimit flag=%s podId=%s iopsdata: %v", flag, podId, iopsdata)
			lmt.In, lmt.InIops = calIopsByBW(flag, iopsdata, total_bw)
		}
	}

	if bw.Wl.OutBps == LimitMax {
		lmt.Out = fmt.Sprintf("%d", max)
		lmt.OutIops = max / 4096
	} else if bw.Wl.OutBps == LimitKeepValue {
		lmt.Out = ""
		lmt.OutIops = 0
	} else {
		if bw.Wl.OutBps < MinPodDiskOutBw {
			bw.Wl.OutBps = MinPodDiskOutBw
		}

		if w.WorkloadType == utils.BeIndex {
			lmt.Out = fmt.Sprintf("%d", int64(bw.Wl.OutBps*utils.MiB))
			lmt.OutIops = int64(bw.Wl.OutBps*utils.MiB) / 32768
		} else {
			total_bw := bw.Wl.OutBps * utils.MiB
			var iopsdata IOPSData
			flag := "write"
			iopsdata.WrReqBW = w.RawRequest[podId].OutBps
			iopsdata.WrTranBW = w.RequestPods[podId].OutBps
			iopsdata.WrTranRatio = iopsdata.WrTranBW / iopsdata.WrReqBW
			iopsdata.WrReqIops = w.RawRequest[podId].OutIops

			klog.V(utils.DBG).Infof("SetAppLimit flag=%s podId=%s iopsdata: %v", flag, podId, iopsdata)
			lmt.Out, lmt.OutIops = calIopsByBW(flag, iopsdata, total_bw)
		}
	}

	klog.V(utils.INF).Infof("SetAppLimit id: %s, In,Iops:%s,%d, Out,Iops:%s,%d", lmt.Id, lmt.In, lmt.InIops, lmt.Out, lmt.OutIops)
	bws = append(bws, &lmt)

	agent.ClientHandlerChan <- &pb.SetAppLimitRequest{AppId: appId, Limit: bws, IoiType: utils.DiskIOIType}
}

func IOIRegisterDisk(podUid string, cgroupPath string, deviceId []string) {
	var diskCgroup utils.DiskCgroupInfo
	diskCgroup.CgroupPath = cgroupPath
	diskCgroup.DeviceId = deviceId
	data, json_err := json.Marshal(diskCgroup)
	if json_err != nil {
		klog.Warningf("Marshal is failed")
	}
	agent.ClientHandlerChan <- &pb.RegisterAppRequest{AppName: podUid, AppInfo: string(data), IoiType: utils.DiskIOIType}
}

func IOIUnRegisterDisk(podUid string) {
	agent.ClientHandlerChan <- &pb.UnRegisterAppRequest{AppId: podUid, IoiType: utils.DiskIOIType}
}

func deepCopyResult(averageResult agent.PodData, previousResult agent.PodData) {
	for disk, podEvent := range averageResult.PodEvents {
		if previousResult.PodEvents[disk] == nil {
			previousResult.PodEvents[disk] = make(map[string]agent.PodRunInfo)
		}
		for pod, podInfo := range podEvent {
			previousResult.PodEvents[disk][pod] = podInfo
		}
	}
}

func compareResult(averageResult agent.PodData, previousResult agent.PodData) bool {
	for disk, podEvent := range averageResult.PodEvents {
		if previousResult.PodEvents[disk] == nil {
			return false
		}
		for pod, podInfo := range podEvent {
			if _, ok := previousResult.PodEvents[disk][pod]; !ok {
				return false
			}
			if math.Abs(podInfo.Actual.InBps-previousResult.PodEvents[disk][pod].Actual.InBps) > 1 ||
				math.Abs(podInfo.Actual.OutBps-previousResult.PodEvents[disk][pod].Actual.OutBps) > 1 {
				return false
			}
		}
	}
	return true
}

func IOISubscribeDisk(DiskInfos map[string]utils.DiskInfo) {
	resultArr := make([]agent.PodData, AverageTime)
	count := 0
	index := 0

	for {
		previousResult := agent.PodData{
			T:         1,
			PodEvents: make(map[string]agent.PodEventInfo),
		}

		stream, err := agent.IoiClient.Subscribe(agent.IoiCtx, &pb.SubscribeContext{
			IoiType: utils.DiskIOIType,
		}, grpc.WaitForReady(true))
		if err != nil {
			klog.Warning("Subscribe error: ", err)
		} else {
			klog.V(utils.DBG).Info("Subscribe succeed")
		}
		for {
			resp, err := stream.Recv()
			if err != nil {
				klog.Warningf("Waiting for subscribe restart,Stream recv failed error: %s", err)
				break
			}

			// update data
			result := parseReceiveMessageDisk(resp, DiskInfos)
			count++
			index = (count - 1) % AverageTime
			resultArr[index] = *result
			averageResult := getAverageResult(resultArr)
			flag := compareResult(averageResult, previousResult)
			if !flag {
				averageResult.T = 1
				deepCopyResult(averageResult, previousResult)
				agent.PodInfoChan <- &averageResult
			}

			// we will return count to 10 after one day
			if count == AverageTime*8640 {
				count = AverageTime
			}
		}
	}
}

type IOPSData struct {
	RdReqBW     float64 // No.1
	RdTranBW    float64 // No.2
	RdTranRatio float64 // No.4
	RdReqIops   float64 // No.3
	WrReqBW     float64
	WrTranBW    float64
	WrReqIops   float64
	WrTranRatio float64
}

func parseReceiveMessageDisk(resp *pb.AppsBandwidth, DiskInfos map[string]utils.DiskInfo) *agent.PodData {
	var result agent.PodData
	podInfos := make(map[string]agent.PodEventInfo)
	var zero []float64
	for i := 0; i < ioiNum; i++ {
		zero = append(zero, 0)
	}

	bsList := []string{"512b", "1k", "4k", "8k", "16k", "32k"}

	appBWInfo := resp.GetDiskIoBw().AppBwInfo
	klog.V(utils.DBG).Infof("resp: %v", resp.String())
	for po, bWInfo := range appBWInfo {
		var disk string
		re := make(map[string][]float64, ioiNum)
		wr := make(map[string][]float64, ioiNum)

		bwi := bWInfo.GetBwInfo()
		for _, bandwidth := range bwi {
			r := make([]float64, ioiNum)
			w := make([]float64, ioiNum)
			di := bandwidth.Id
			disk = ""
			for id, info := range DiskInfos {
				if info.MajorMinor == di {
					disk = id
					break
				}
			}
			if disk == "" {
				klog.Warning("no such disk found")
				continue
			}
			bws := bandwidth.Bandwidths
			if bandwidth.IsRead {
				r[0], _ = strconv.ParseFloat(bws["512b"], 32)
				r[1], _ = strconv.ParseFloat(bws["1k"], 32)
				r[2], _ = strconv.ParseFloat(bws["4k"], 32)
				r[3], _ = strconv.ParseFloat(bws["8k"], 32)
				r[4], _ = strconv.ParseFloat(bws["16k"], 32)
				r[5], _ = strconv.ParseFloat(bws["32k"], 32)
				for i := 0; i < ioiNum; i++ {
					r[i] = r[i] / utils.MiB
				}
				re[disk] = r
			} else {
				w[0], _ = strconv.ParseFloat(bws["512b"], 32)
				w[1], _ = strconv.ParseFloat(bws["1k"], 32)
				w[2], _ = strconv.ParseFloat(bws["4k"], 32)
				w[3], _ = strconv.ParseFloat(bws["8k"], 32)
				w[4], _ = strconv.ParseFloat(bws["16k"], 32)
				w[5], _ = strconv.ParseFloat(bws["32k"], 32)
				for i := 0; i < ioiNum; i++ {
					w[i] = w[i] / utils.MiB
				}
				wr[disk] = w
			}
		}

		var reSum, wrSum, reSumOrig, wrSumOrig float64
		for devId, devInfo := range DiskInfos {
			var wl utils.Workload
			wlBs := make(map[string]*utils.Workload)

			reSum = 0
			wrSum = 0
			for i := 0; i < ioiNum; i++ {
				if re[devId] == nil {
					re[devId] = zero
				}
				if wr[devId] == nil {
					wr[devId] = zero
				}
				if re[devId][i] != 0 {
					reSum += devInfo.ReadRatio[bsList[i]] * re[devId][i]
					reSumOrig += re[devId][i]
				}
				if wr[devId][i] != 0 {
					wrSum += devInfo.WriteRatio[bsList[i]] * wr[devId][i]
					wrSumOrig += wr[devId][i]
				}
				wlBs[bsList[i]] = &utils.Workload{
					InBps:    re[devId][i],
					OutBps:   wr[devId][i],
					TotalBps: re[devId][i] + wr[devId][i],
				}
			}
			wl.InBps = reSum
			wl.OutBps = wrSum

			podRunInfo := agent.PodRunInfo{
				Actual:      &wl,
				RawActualBs: &wlBs,
			}
			if podInfos[devId] == nil {
				podInfos[devId] = make(map[string]agent.PodRunInfo)
			}
			podInfos[devId][po] = podRunInfo
		}
	}

	result.T = 1
	result.PodEvents = podInfos

	return &result
}

func IOIGetServiceInfoDisk(req string) *agent.AllProfile {
	var allProfile agent.AllProfile
	var diskInfos map[string]utils.BlockDevice

	if agent.IoiClient == nil {
		klog.Errorf("ioid is not set!")
		return nil
	}

	ServiceInfoResponse, err := agent.IoiClient.GetServiceInfo(agent.IoiCtx, &pb.ServiceInfoRequest{IoiType: utils.DiskIOIType, Req: req})
	if err != nil {
		klog.Warning("GetServiceInfo failed, please wait for server to start:", err)
		return nil
	} else {
		klog.V(utils.INF).Infof("GetServiceInfo success GetServiceInfo %v\n", ServiceInfoResponse.Configuration)
		err = json.Unmarshal([]byte(ServiceInfoResponse.Configuration), &diskInfos)
		if err == nil {
			allProfile.T = 0
			allProfile.DInfo = diskInfos
		} else {
			klog.Errorf("parse ServiceInfoResponse failed %s due to %v\n", ServiceInfoResponse.Configuration, err)
			return nil
		}
	}
	return &allProfile
}
