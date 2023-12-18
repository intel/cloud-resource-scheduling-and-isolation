/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package service

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
)

type TestEngine struct {
	ServiceEngine
	Interval utils.IntervalConfigInfo // current configuration interval
}

func (t *TestEngine) Type() int32 {
	return 5
}

func (t *TestEngine) Initialize(persist common.IPersist) error {
	klog.V(utils.INF).Info("persistent: file open")
	return nil
}

func (t *TestEngine) Uninitialize() error {
	klog.V(utils.INF).Info("persistent: file close")
	return nil
}

func (t *TestEngine) Configure(ctx context.Context, req *pb.ConfigureRequest) (*pb.ConfigureResponse, error) {
	return &pb.ConfigureResponse{}, nil
}

func (t *TestEngine) RegisterApp(ctx context.Context, req *pb.RegisterAppRequest) (*pb.RegisterAppResponse, error) {
	return &pb.RegisterAppResponse{
		AppId:  req.AppName,
		Status: pb.ResponseState_OK,
	}, nil
}

func (t *TestEngine) UnRegisterApp(ctx context.Context, req *pb.UnRegisterAppRequest) (*pb.UnRegisterAppResponse, error) {

	return &pb.UnRegisterAppResponse{}, nil
}

func (t *TestEngine) SetAppLimit(ctx context.Context, req *pb.SetAppLimitRequest) (*pb.SetAppLimitResponse, error) {
	return &pb.SetAppLimitResponse{}, nil
}

func (t *TestEngine) Subscribe(req *pb.SubscribeContext, stream pb.Ioiservice_SubscribeServer) error {
	return nil
}

func (t *TestEngine) GetServiceInfo(ctx context.Context, req *pb.ServiceInfoRequest) (*pb.ServiceInfoResponse, error) {
	return &pb.ServiceInfoResponse{}, nil
}

func TestIOIServer_Start(t *testing.T) {
	type fields struct {
		UnimplementedIoiserviceServer pb.UnimplementedIoiserviceServer
		Name                          string
		se                            *Service
	}
	type args struct {
		Persist common.IPersist
	}
	pt1 := gomonkey.ApplyGlobalVar(&common.StoreDirectory, "../../build/")
	defer pt1.Reset()
	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../build/test2.db")
	defer pt2.Reset()

	db := common.NewDBPersist()
	err := db.CreateTable("COMMON", []string{})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}

	a := GetService()
	engine := &TestEngine{
		ServiceEngine{},
		utils.IntervalConfigInfo{
			Interval: 1,
		},
	}

	a.RegisterEngine(engine)

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test ioi server",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test",
				se:                            &Service{},
			},
			args: args{
				Persist: db,
			},
			wantErr: false,
		},
		{
			name: "test ioi server with error",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test",
				se:                            a,
			},
			args: args{
				Persist: db,
			},
			wantErr: false,
		},
	}

	defer os.Remove("../../../build/test2.db")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &IOIServer{
				UnimplementedIoiserviceServer: tt.fields.UnimplementedIoiserviceServer,
				Name:                          tt.fields.Name,
				se:                            tt.fields.se,
			}
			if err := s.Start(tt.args.Persist); (err != nil) != tt.wantErr {
				t.Errorf("IOIServer.Start() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := s.Stop(); (err != nil) != tt.wantErr {
				t.Errorf("IOIServer.Stop() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIOIServer_SetAppLimit(t *testing.T) {
	type fields struct {
		UnimplementedIoiserviceServer pb.UnimplementedIoiserviceServer
		Name                          string
		se                            *Service
	}
	type args struct {
		ctx context.Context
		req *pb.SetAppLimitRequest
	}

	a := GetService()
	engine := &TestEngine{
		ServiceEngine{},
		utils.IntervalConfigInfo{
			Interval: 1,
		},
	}

	a.RegisterEngine(engine)

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.SetAppLimitResponse
		wantErr bool
	}{
		{
			name: "test ioi server SetAppLimit",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test",
				se:                            &Service{},
			},
			args: args{
				ctx: context.Background(),
				req: &pb.SetAppLimitRequest{},
			},
			want:    &pb.SetAppLimitResponse{Status: pb.ResponseState_FAIL},
			wantErr: true,
		},
		{
			name: "test ioi server SetAppLimit for test engine",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test1",
				se:                            a,
			},
			args: args{
				ctx: context.Background(),
				req: &pb.SetAppLimitRequest{IoiType: 5},
			},
			want:    &pb.SetAppLimitResponse{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &IOIServer{
				UnimplementedIoiserviceServer: tt.fields.UnimplementedIoiserviceServer,
				Name:                          tt.fields.Name,
				se:                            tt.fields.se,
			}
			got, err := s.SetAppLimit(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("IOIServer.SetAppLimit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IOIServer.SetAppLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIOIServer_UnRegisterApp(t *testing.T) {
	type fields struct {
		UnimplementedIoiserviceServer pb.UnimplementedIoiserviceServer
		Name                          string
		se                            *Service
	}
	type args struct {
		ctx context.Context
		req *pb.UnRegisterAppRequest
	}

	a := GetService()
	engine := &TestEngine{
		ServiceEngine{},
		utils.IntervalConfigInfo{
			Interval: 1,
		},
	}

	a.RegisterEngine(engine)

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.UnRegisterAppResponse
		wantErr bool
	}{
		{
			name: "test ioi server UnRegisterApp",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test",
				se:                            &Service{},
			},
			args: args{
				ctx: context.Background(),
				req: &pb.UnRegisterAppRequest{},
			},
			want:    &pb.UnRegisterAppResponse{Status: pb.ResponseState_FAIL},
			wantErr: true,
		},
		{
			name: "test ioi server UnRegisterApp for test engine",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test1",
				se:                            a,
			},
			args: args{
				ctx: context.Background(),
				req: &pb.UnRegisterAppRequest{IoiType: 5},
			},
			want:    &pb.UnRegisterAppResponse{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &IOIServer{
				UnimplementedIoiserviceServer: tt.fields.UnimplementedIoiserviceServer,
				Name:                          tt.fields.Name,
				se:                            tt.fields.se,
			}
			got, err := s.UnRegisterApp(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("IOIServer.UnRegisterApp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IOIServer.UnRegisterApp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIOIServer_RegisterApp(t *testing.T) {
	type fields struct {
		UnimplementedIoiserviceServer pb.UnimplementedIoiserviceServer
		Name                          string
		se                            *Service
	}
	type args struct {
		ctx context.Context
		req *pb.RegisterAppRequest
	}

	a := GetService()
	engine := &TestEngine{
		ServiceEngine{},
		utils.IntervalConfigInfo{
			Interval: 1,
		},
	}

	a.RegisterEngine(engine)

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.RegisterAppResponse
		wantErr bool
	}{
		{
			name: "test ioi server RegisterApp",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test",
				se:                            &Service{},
			},
			args: args{
				ctx: context.Background(),
				req: &pb.RegisterAppRequest{},
			},
			want:    &pb.RegisterAppResponse{Status: pb.ResponseState_FAIL},
			wantErr: true,
		},
		{
			name: "test ioi server RegisterApp for test engine",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test1",
				se:                            a,
			},
			args: args{
				ctx: context.Background(),
				req: &pb.RegisterAppRequest{IoiType: 5},
			},
			want:    &pb.RegisterAppResponse{AppId: "", Status: pb.ResponseState_OK},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &IOIServer{
				UnimplementedIoiserviceServer: tt.fields.UnimplementedIoiserviceServer,
				Name:                          tt.fields.Name,
				se:                            tt.fields.se,
			}
			got, err := s.RegisterApp(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("IOIServer.RegisterApp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IOIServer.RegisterApp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIOIServer_Subscribe(t *testing.T) {
	type fields struct {
		UnimplementedIoiserviceServer pb.UnimplementedIoiserviceServer
		Name                          string
		se                            *Service
	}
	type args struct {
		req    *pb.SubscribeContext
		stream pb.Ioiservice_SubscribeServer
	}

	a := GetService()
	engine := &TestEngine{
		ServiceEngine{},
		utils.IntervalConfigInfo{
			Interval: 1,
		},
	}

	a.RegisterEngine(engine)

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test ioi server Subscribe",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test",
				se:                            &Service{},
			},
			args: args{
				req:    &pb.SubscribeContext{},
				stream: nil,
			},
			wantErr: false,
		},
		{
			name: "test ioi server Subscribe for test engine",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test1",
				se:                            a,
			},
			args: args{
				req:    &pb.SubscribeContext{IoiType: 5},
				stream: nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &IOIServer{
				UnimplementedIoiserviceServer: tt.fields.UnimplementedIoiserviceServer,
				Name:                          tt.fields.Name,
				se:                            tt.fields.se,
			}
			if err := s.Subscribe(tt.args.req, tt.args.stream); (err != nil) != tt.wantErr {
				t.Errorf("IOIServer.Subscribe() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewIoisolationServer(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "test new ioi server",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIoisolationServer()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewIoisolationServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestIOIServer_Configure(t *testing.T) {
	type fields struct {
		UnimplementedIoiserviceServer pb.UnimplementedIoiserviceServer
		Name                          string
		se                            *Service
	}
	type args struct {
		ctx context.Context
		req *pb.ConfigureRequest
	}

	a := GetService()
	engine := &TestEngine{
		ServiceEngine{},
		utils.IntervalConfigInfo{
			Interval: 1,
		},
	}

	a.RegisterEngine(engine)

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.ConfigureResponse
		wantErr bool
	}{
		{
			name: "test ioi server Configure",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test",
				se:                            &Service{},
			},
			args: args{
				ctx: context.Background(),
				req: &pb.ConfigureRequest{},
			},
			want:    &pb.ConfigureResponse{Status: pb.ResponseState_FAIL},
			wantErr: true,
		},
		{
			name: "test ioi server Configure for test engine",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test1",
				se:                            a,
			},
			args: args{
				ctx: context.Background(),
				req: &pb.ConfigureRequest{IoiType: 5},
			},
			want:    &pb.ConfigureResponse{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &IOIServer{
				UnimplementedIoiserviceServer: tt.fields.UnimplementedIoiserviceServer,
				Name:                          tt.fields.Name,
				se:                            tt.fields.se,
			}
			got, err := s.Configure(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("IOIServer.Configure() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IOIServer.Configure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIOIServer_GetServiceInfo(t *testing.T) {
	type fields struct {
		UnimplementedIoiserviceServer pb.UnimplementedIoiserviceServer
		Name                          string
		se                            *Service
	}
	type args struct {
		ctx context.Context
		req *pb.ServiceInfoRequest
	}

	a := GetService()
	engine := &TestEngine{
		ServiceEngine{},
		utils.IntervalConfigInfo{
			Interval: 1,
		},
	}

	a.RegisterEngine(engine)

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.ServiceInfoResponse
		wantErr bool
	}{
		{
			name: "test ioi server GetServiceInfo",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test",
				se:                            &Service{},
			},
			args: args{
				ctx: context.Background(),
				req: &pb.ServiceInfoRequest{},
			},
			want:    &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL},
			wantErr: true,
		},
		{
			name: "test ioi server GetServiceInfo for test engine",
			fields: fields{
				UnimplementedIoiserviceServer: pb.UnimplementedIoiserviceServer{},
				Name:                          "test1",
				se:                            a,
			},
			args: args{
				ctx: context.Background(),
				req: &pb.ServiceInfoRequest{IoiType: 5},
			},
			want:    &pb.ServiceInfoResponse{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &IOIServer{
				UnimplementedIoiserviceServer: tt.fields.UnimplementedIoiserviceServer,
				Name:                          tt.fields.Name,
				se:                            tt.fields.se,
			}
			got, err := s.GetServiceInfo(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("IOIServer.GetServiceInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IOIServer.GetServiceInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}
