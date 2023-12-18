/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"

	"google.golang.org/grpc/backoff"
	utils "sigs.k8s.io/IOIsolation/pkg"
	aggr "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	agent "sigs.k8s.io/IOIsolation/pkg/api/nodeagent"
)

const (
	port = 9988
)

var (
	client       aggr.IOAggregator4AgentClient
	ctx          context.Context
	CrPodRequest map[string]*aggr.PodRequest
)

// use when node agent restarts
func GetReservedPods(nodeName string) {
	if client == nil {
		klog.Warning("agent client is nil, GetReservedPods failed")
		return
	}
	res, err := client.GetReservedPods(ctx, &aggr.GetReservedPodsRequest{NodeName: nodeName})
	if err != nil {
		klog.Warning("GetReservedPods failed, err: ", err)
		return
	}

	resPods := res.PodList
	klog.V(utils.INF).Info("now in GetReservedPods:", resPods)
	pDiskUpdate, pNetUpdate, pRdtUpdate, pRdtQuantityUpdate := ProcessPodsFromAggregator(resPods)
	PodInfoChan <- pDiskUpdate
	PodInfoChan <- pNetUpdate
	PodInfoChan <- pRdtUpdate
	PodInfoChan <- pRdtQuantityUpdate
}

func ProcessPodsFromAggregator(resPods *aggr.PodList) (*PodData, *PodData, *PodData, *PodData) {
	diskUpdate := PodData{
		2,
		resPods.Generation,
		make(map[string]PodEventInfo),
	}
	netUpdate := PodData{
		4,
		resPods.Generation,
		make(map[string]PodEventInfo),
	}
	rdtUpdate := PodData{
		5,
		resPods.Generation,
		make(map[string]PodEventInfo),
	}
	rdtQuantityUpdate := PodData{
		6,
		resPods.Generation,
		make(map[string]PodEventInfo),
	}
	if len(resPods.PodList) == 0 {
		klog.V(utils.INF).Info("ProcessPodsFromAggregator: Get 0 Pod from aggregator")
		return &diskUpdate, &netUpdate, &rdtUpdate, &rdtQuantityUpdate
	} else {
		for podUid, podRequest := range resPods.PodList {
			if podRequest != nil && len(podRequest.Request) > 0 {
				CrPodRequest[podUid] = podRequest
				klog.V(utils.INF).Infof("ProcessPodsFromAggregator: podRequest CrPodRequest is exist pod =%s", podUid)
			} else {
				if pr, ok := CrPodRequest[podUid]; ok {
					klog.V(utils.INF).Infof("ProcessPodsFromAggregator: CrPodRequest is exist pod =%s", podUid)
					podRequest = pr
				} else {
					klog.V(utils.INF).Infof("ProcessPodsFromAggregator: CrPodRequest is not exist pod =%s", podUid)
					continue
				}
			}

			for _, request := range podRequest.Request {
				pDevReq := request.DevRequest
				devName := pDevReq.DevName
				ReqWl := utils.Workload{
					InBps:   pDevReq.Request.Inbps,
					OutBps:  pDevReq.Request.Outbps,
					InIops:  pDevReq.Request.Iniops,
					OutIops: pDevReq.Request.Outiops,
				}
				LimitWl := utils.Workload{
					InBps:   pDevReq.Limit.Inbps,
					OutBps:  pDevReq.Limit.Outbps,
					InIops:  pDevReq.Limit.Iniops,
					OutIops: pDevReq.Limit.Outiops,
				}
				RawReqWl := utils.Workload{
					InBps:   pDevReq.RawRequest.Inbps,
					OutBps:  pDevReq.RawRequest.Outbps,
					InIops:  pDevReq.RawRequest.Iniops,
					OutIops: pDevReq.RawRequest.Outiops,
				}
				RawLimitWl := utils.Workload{
					InBps:   pDevReq.RawLimit.Inbps,
					OutBps:  pDevReq.RawLimit.Outbps,
					InIops:  pDevReq.RawLimit.Iniops,
					OutIops: pDevReq.RawLimit.Outiops,
				}
				wlType := request.WorkloadType
				klog.V(utils.INF).Infof("ProcessPodsFromAggregator: Request:%+v, Limit:%+v", ReqWl, LimitWl)
				klog.V(utils.DBG).Infof("ProcessPodsFromAggregator: RawReq:%+v, RawLmt:%+v", RawReqWl, RawLimitWl)

				podRunInfo := PodRunInfo{
					Request:      &ReqWl,
					Limit:        &LimitWl,
					RawRequest:   &RawReqWl,
					RawLimit:     &RawLimitWl,
					WorkloadType: utils.BeIndex,
					PodName:      podRequest.PodName,
				}

				// GA, BT, BE
				if wlType != utils.BeIndex {
					pReq := podRunInfo.Request
					pLmt := podRunInfo.Limit
					if pReq.InBps == pLmt.InBps && pReq.OutBps == pLmt.OutBps && pReq.InIops == pLmt.InIops && pReq.OutIops == pLmt.OutIops {
						podRunInfo.WorkloadType = utils.GaIndex
					} else {
						podRunInfo.WorkloadType = utils.BtIndex
					}
				} else {
					podRunInfo.WorkloadType = utils.BeIndex
				}

				if request.IoType == aggr.IOType_DiskIO {
					if diskUpdate.PodEvents[devName] == nil {
						diskUpdate.PodEvents[devName] = make(map[string]PodRunInfo)
					}
					diskUpdate.PodEvents[devName][podUid] = podRunInfo
				} else if request.IoType == aggr.IOType_NetworkIO {
					if netUpdate.PodEvents[devName] == nil {
						netUpdate.PodEvents[devName] = make(map[string]PodRunInfo)
					}
					netUpdate.PodEvents[devName][podUid] = podRunInfo
				} else if request.IoType == aggr.IOType_RDTQuantity {
					podRunInfo.WorkloadType = utils.GaIndex
					if rdtQuantityUpdate.PodEvents[devName] == nil {
						rdtQuantityUpdate.PodEvents[devName] = make(map[string]PodRunInfo)
					}
					rdtQuantityUpdate.PodEvents[devName][podUid] = podRunInfo
				}
			}
		}
		klog.V(utils.INF).Infof("ProcessPodsFromAggregator: update disk:%v\n", diskUpdate)
		klog.V(utils.INF).Infof("ProcessPodsFromAggregator: update net:%v\n", netUpdate)
		klog.V(utils.INF).Infof("ProcessPodsFromAggregator: update rdt:%v\n", rdtUpdate)
		klog.V(utils.INF).Infof("ProcessPodsFromAggregator: update rdtQuantity:%v\n", rdtQuantityUpdate)

		return &diskUpdate, &netUpdate, &rdtUpdate, &rdtQuantityUpdate
	}
}

func UpdateNodeIOStatus(nodeName string, IO *aggr.IOBandwidth, gen int64) error {
	var bw []*aggr.IOBandwidth
	bw = append(bw, IO)
	retry_time := [3]time.Duration{1, 2, 0}
	var err error

	if client != nil {
		for i := 0; i < len(retry_time); i++ {
			_, err = client.UpdateNodeIOStatus(ctx, &aggr.UpdateNodeIOStatusRequest{NodeName: nodeName, ObservedGeneration: gen, IoBw: bw})
			if err == nil {
				klog.V(utils.DBG).Info("UpdateNodeIOStatus successfully!")
				return nil
			}
			klog.Warning("UpdateNodeIOStatus failed, err: ", err)
			time.Sleep(time.Second * retry_time[i])
		}
		klog.Warning("UpdateNodeIOStatus retry three times failed, err: ", err)
		return err
	} else {
		err := errors.New("client is nil, UpdateNodeIOStatus failed")
		klog.Warning("client is nil, UpdateNodeIOStatus failed")
		return err
	}
}

func GetCertServer() (string, string, string) {
	return CACertFile, AgentKeyFile, AgentCertFile
}

func GetAgentServer(nodeName string) *AgentServer {
	srv := &AgentServer{
		Name: nodeName,
	}
	return srv
}

func GetAggregatorPort() int {
	return AggregatorPort
}

func (c *Agent) StartServer4Aggregator() {
	port := GetAggregatorPort()
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		klog.Fatalf("failed to start listening %s: %v", c.IOInfo.Spec.EndPoint, err)
	}
	var creds credentials.TransportCredentials
	certFile, agentKeyFile, agentCertFile := GetCertServer()
	if c.mtls {
		creds = utils.LoadKeyPair(filepath.Join(utils.TlsPath, certFile), agentKeyFile, agentCertFile, true)
	} else {
		creds = insecure.NewCredentials()
	}
	s := grpc.NewServer(grpc.Creds(creds))
	reflection.Register(s)
	srv := GetAgentServer(c.IOInfo.Spec.NodeName)
	agent.RegisterNodeAgentServer(s, srv)

	// Log messages
	klog.Info("4. Node agent aggregator server is start.")
	err = s.Serve(l)
	if err != nil {
		klog.Fatalf("failed to serve: %v", err)
		os.Exit(1)
	}
}

func (c *Agent) StartClient4Aggregator(nodeName string) error {
	var errorClient error

	addr := fmt.Sprintf("aggregator.ioi-system.svc.cluster.local:%d", port)

	retryPolicy := `{
		"methodConfig": [{
		  "name": [
		    {"service": "aggregator.IOAggregator4Agent","method":"UpdateNodeIOStatus"},
			{"service": "aggregator.IOAggregator4Agent","method":"GetReservedPods"}
		  ],
		  "retryPolicy": {
			"MaxAttempts": 3,
			"InitialBackoff": "1s",
			"MaxBackoff": "3s",
			"BackoffMultiplier": 1.5,
			"RetryableStatusCodes": [ "UNAVAILABLE" ]
		  }
		}]}`
	var creds credentials.TransportCredentials
	certFile, agentKeyFile, agentCertFile := GetCertServer()
	if c.mtls {
		creds = utils.LoadKeyPair(filepath.Join(utils.TlsPath, certFile), agentKeyFile, agentCertFile, false)
	} else {
		creds = insecure.NewCredentials()
	}
	netConn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds), grpc.WithDefaultServiceConfig(retryPolicy), grpc.WithConnectParams(grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  time.Second,
			Multiplier: 1.5,
			MaxDelay:   60 * time.Second,
		}}))
	if err != nil {
		klog.Error("node agent failed to connect to aggregator, error: ", err)
		return errorClient
	} else {
		klog.V(utils.DBG).Info("node agent successfully connected to aggregator")
	}

	client = aggr.NewIOAggregator4AgentClient(netConn)
	if client == nil {
		klog.Error("Failed to create agent client error: ", err)
		return err
	} else {
		klog.V(utils.DBG).Info("Successfully create agent client")
	}
	// Contact the server and print out its response.
	ctx = context.Background()
	c.aggregatorConn = netConn

	GetReservedPods(nodeName)

	return nil
}

// ---- Agent Server for Aggregator  -----
type AgentServer struct {
	agent.UnimplementedNodeAgentServer
	Name string
}

// UpdateReservedPod update pods in node agent from aggregator
func (s *AgentServer) UpdateReservedPods(ctx context.Context, req *agent.UpdateReservedPodsRequest) (*aggr.Empty, error) {
	klog.V(utils.INF).Info("now in UpdateReservedPods:", req.ReservedPods)
	if req.NodeName != s.Name {
		return &aggr.Empty{}, fmt.Errorf("the node name: %v in request is not consistent with the one in AgentServer", req.NodeName)
	}
	resPods := req.ReservedPods

	pDiskUpdate, pNetUpdate, pRdtUpdate, pRdtQuantityUpdate := ProcessPodsFromAggregator(resPods)
	PodInfoChan <- pDiskUpdate
	PodInfoChan <- pNetUpdate
	PodInfoChan <- pRdtUpdate
	PodInfoChan <- pRdtQuantityUpdate

	return &aggr.Empty{}, nil
}

func init() {
	CrPodRequest = make(map[string]*aggr.PodRequest)
}
