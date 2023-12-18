/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
)

const (
	DiskSwitch        = 0b1
	NetSwitch         = 0b10
	RDTSwitch         = 0b100
	RDTQuantitySwitch = 0b1000
)

type IOIServer struct {
	pb.UnimplementedIoiserviceServer
	Name string
	se   *Service
}

func NewIoisolationServer() (*IOIServer, error) {
	srv := &IOIServer{
		se: GetService(),
	}
	return srv, nil
}

func (s *IOIServer) Start(Persist common.IPersist) error {
	commonBuckets := []string{}
	err := Persist.CreateTable("COMMON", commonBuckets)
	if err != nil {
		klog.Warningf("Create net buckets fail: %s", err)
	}

	s.se.Persist = Persist

	LogLevel, err := common.LoadLogLevel(Persist)
	if err != nil {
		LogLevel = utils.LogLevelConfigInfo{Info: true, Debug: true}
		klog.Errorf("getConfigLog %v from store error: %v", LogLevel, err)
	} else {
		klog.V(utils.DBG).Infof("persistent: rebuild configLog: %v", LogLevel)
	}
	utils.SetLogLevel(LogLevel.Info, LogLevel.Debug)

	// initialize data engines
	for _, engine := range s.se.engines {
		err := engine.Initialize(Persist)
		if err != nil {
			klog.Warning("1.service initialize engine failed, error:", err)
		} else {
			klog.Info("2.service initialize engine succeeded, engine:", engine.Type())
		}
	}

	return nil
}

func (s *IOIServer) Stop() error {
	// close data engines
	for _, engine := range s.se.engines {
		err := engine.Uninitialize()
		if err != nil {
			klog.Warning("1.service Uninitialize engine failed, error:", err)
		} else {
			klog.Info("2.service Uninitialize engine succeeded, engine:", engine.Type())
		}
	}

	// close database
	s.se.Persist.Close()

	return nil
}

func (s *IOIServer) Configure(ctx context.Context, req *pb.ConfigureRequest) (*pb.ConfigureResponse, error) {
	if req.IoiType == utils.CommonIOIType {
		if req.Type == utils.LOGLEVEL {
			if req.Configure == "" {
				return &pb.ConfigureResponse{Status: pb.ResponseState_FAIL}, nil
			}
			var loglevel utils.LogLevelConfigInfo
			err := json.Unmarshal([]byte(req.Configure), &loglevel)
			if err == nil {
				klog.Infof("parseConfigureRequest success %v\n", loglevel)
			} else {
				klog.Errorf("parseConfigureRequest failed because string format is wrong, %s\n", err)
				return &pb.ConfigureResponse{Status: pb.ResponseState_FAIL}, nil
			}
			klog.Infof("Configure loglevel %v.", loglevel)
			utils.SetLogLevel(loglevel.Info, loglevel.Debug)
			err = common.SaveLogLevel(s.se.Persist, loglevel)
			if err != nil {
				klog.Errorf("Configure %s", err)
				return nil, status.Error(codes.Unimplemented, "update db for config failed")
			}
			return &pb.ConfigureResponse{Status: pb.ResponseState_OK}, nil
		} else {
			return &pb.ConfigureResponse{Status: pb.ResponseState_FAIL}, errors.New("ioi type is invalid")
		}
	} else {
		if engine, ok := s.se.engines[req.IoiType]; ok {
			configResponse, err := engine.Configure(ctx, req)
			if err != nil {
				klog.Warning("1.service configure engine failed, error:", err)
			} else {
				klog.Info("2.service configure engine succeeded, engine:", engine.Type())
			}

			return configResponse, nil

		} else {
			return &pb.ConfigureResponse{Status: pb.ResponseState_FAIL}, errors.New("with engine can configure it")
		}
	}
}

func (s *IOIServer) SetAppLimit(ctx context.Context, req *pb.SetAppLimitRequest) (*pb.SetAppLimitResponse, error) {
	if engine, ok := s.se.engines[req.IoiType]; ok {
		limitResponse, err := engine.SetAppLimit(ctx, req)
		if err != nil {
			klog.Warning("1.service SetAppLimit engine failed, error:", err)
		} else {
			klog.V(utils.INF).Info("2.service SetAppLimit engine succeeded, engine:", engine.Type())
		}

		return limitResponse, nil
	} else {
		return &pb.SetAppLimitResponse{Status: pb.ResponseState_FAIL}, errors.New("no engine can set app limit it")
	}
}

func (s *IOIServer) UnRegisterApp(ctx context.Context, req *pb.UnRegisterAppRequest) (*pb.UnRegisterAppResponse, error) {
	if engine, ok := s.se.engines[req.IoiType]; ok {
		UnregisterResponse, err := engine.UnRegisterApp(ctx, req)
		if err != nil {
			klog.Warning("1.service UnRegisterApp engine failed, error:", err)
		} else {
			klog.Info("2.service UnRegisterApp engine succeeded, engine:", engine.Type())
		}

		return UnregisterResponse, nil
	} else {
		return &pb.UnRegisterAppResponse{Status: pb.ResponseState_FAIL}, errors.New("no engine can unregister")
	}
}

func (s *IOIServer) RegisterApp(ctx context.Context, req *pb.RegisterAppRequest) (*pb.RegisterAppResponse, error) {
	if engine, ok := s.se.engines[req.IoiType]; ok {
		registerResponse, err := engine.RegisterApp(ctx, req)
		if err != nil {
			klog.Warning("1.service RegisterApp engine failed, error:", err)
		} else {
			klog.Info("2.service RegisterApp engine succeeded, engine:", engine.Type())
		}

		return registerResponse, nil
	} else {
		return &pb.RegisterAppResponse{Status: pb.ResponseState_FAIL}, errors.New("no engine can register")
	}
}

func (s *IOIServer) Subscribe(req *pb.SubscribeContext, stream pb.Ioiservice_SubscribeServer) error {
	if engine, ok := s.se.engines[req.IoiType]; ok {
		err := engine.Subscribe(req, stream)
		if err != nil {
			klog.Warning("1.service Subscribe engine failed, error:", err)
		} else {
			klog.Info("2.service Subscribe engine succeeded, engine:", engine.Type())
		}

		return err
	} else {
		return nil
	}
}

func (s *IOIServer) GetServiceInfo(ctx context.Context, req *pb.ServiceInfoRequest) (*pb.ServiceInfoResponse, error) {
	if engine, ok := s.se.engines[req.IoiType]; ok {
		resp, err := engine.GetServiceInfo(ctx, req)
		if err != nil {
			klog.Warning("1.service GetServiceInfo engine failed, error:", err)
		} else {
			klog.Info("2.service GetServiceInfo engine succeeded, engine:", engine.Type())
		}

		return resp, err
	} else {
		return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, errors.New("no engine can get service info")
	}
}

// Service
type Service struct {
	engines       map[int32]IEngine
	EnginesSwitch int
	srv           *IOIServer
	Persist       common.IPersist
}

var node_service = Service{
	engines: make(map[int32]IEngine),
	Persist: common.NewDBPersist(),
}

func GetService() *Service {
	return &node_service
}

func (s *Service) RegisterEngine(engine IEngine) {
	s.engines[engine.Type()] = engine
}

func (s *Service) Close() error {
	err := s.srv.Stop()
	if err != nil {
		klog.Warning("1.service close engine failed, error:", err)
		return err
	}

	return nil
}

func (s *Service) Run() error {
	// start service server for node agent
	go func() {
		l, err := net.Listen("unix", pb.IOISockPath)
		if err != nil {
			log.Fatalf("failed to start to %v", err)
		}
		IsTls := viper.GetBool("is_tls")

		var creds credentials.TransportCredentials
		if IsTls {
			creds, err = credentials.NewServerTLSFromFile(pb.TlsKeyDir+pb.ServerCrt,
				pb.TlsKeyDir+pb.ServerKey)
			if err != nil {
				log.Fatalf("Failed to generate credentials %v", err)
			}
		} else {
			creds = insecure.NewCredentials()
		}
		gs := grpc.NewServer(grpc.Creds(creds))
		srv, err := NewIoisolationServer()
		if err != nil {
			klog.Fatalf("failed to create server: %v", err)
		}
		s.srv = srv
		err = srv.Start(s.Persist)
		if err != nil {
			klog.Fatalf("failed to start server: %v", err)
			return
		}

		pb.RegisterIoiserviceServer(gs, srv)

		err = gs.Serve(l)
		if err != nil {
			log.Fatalf("failed to serve: %v", err)
		} else {
			klog.Info("Client connected")
		}

		err = srv.Stop()
		if err != nil {
			klog.Warning("1.service close engine failed, error:", err)
		}
	}()

	return nil
}

func init() {
	klog.Info("0.Init agent client")
	c := GetService()
	viper.SetConfigName("ioi-service.toml")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/etc/ioi-service/")
	err := viper.ReadInConfig()
	if err != nil {
		klog.Warningf("read config failed: %v", err)
	}

	labels := viper.GetString("node_label")
	klog.Infof("get node label: %s", labels)
	c.EnginesSwitch = 0
	resources := strings.Split(labels, "-")

	for _, r := range resources {
		if r == "disk" || r == "all" {
			c.EnginesSwitch = (c.EnginesSwitch | DiskSwitch)
		}
		if r == "net" || r == "all" {
			c.EnginesSwitch = (c.EnginesSwitch | NetSwitch)
		}
		if r == "rdt" || r == "all" {
			c.EnginesSwitch = (c.EnginesSwitch | RDTSwitch)
		}
		if r == "rdtQuantity" || r == "all" {
			c.EnginesSwitch = (c.EnginesSwitch | RDTQuantitySwitch)
		}
	}
}
