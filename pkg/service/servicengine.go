/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package service

import (
	"context"

	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
)

// IEngine
type IEngine interface {
	Type() int32
	Initialize(persist common.IPersist) error
	Uninitialize() error
	Configure(ctx context.Context, req *pb.ConfigureRequest) (*pb.ConfigureResponse, error)
	SetAppLimit(ctx context.Context, req *pb.SetAppLimitRequest) (*pb.SetAppLimitResponse, error)
	UnRegisterApp(ctx context.Context, req *pb.UnRegisterAppRequest) (*pb.UnRegisterAppResponse, error)
	RegisterApp(ctx context.Context, req *pb.RegisterAppRequest) (*pb.RegisterAppResponse, error)
	Subscribe(req *pb.SubscribeContext, stream pb.Ioiservice_SubscribeServer) error
	GetServiceInfo(ctx context.Context, req *pb.ServiceInfoRequest) (*pb.ServiceInfoResponse, error)
}

type ServiceEngine struct {
	pb.UnimplementedIoiserviceServer
	Name string
}
