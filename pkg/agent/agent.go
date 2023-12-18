/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	v11 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
)

const (
	CrNamespace            = "ioi-system"
	AggregatorPort         = 8997
	IoiLabel               = "ioisolation"
	CsiSeperatedLabel      = "csi-seperated"
	DiskSwitch             = 0b1
	NetSwitch              = 0b10
	RDTSwitch              = 0b100
	RDTQuantitySwitch      = 0b1000
	CSISwitch              = 0b10000
	CommonName             = "NodeAgentServer"
	CACommonName           = "NodeAgentCA"
	ServiceCommonName      = "NodeAgentService"
	CAKeyFile              = "root-ca.key"
	CACertFile             = "root-ca.crt"
	AgentKeyFile           = "/etc/ioi/control/pki/na-server.key"
	AgentCertFile          = "/etc/ioi/control/pki/na-server.crt"
	AgentServiceCAKeyFile  = "/etc/ioi/service/pki/na-service-ca.key"
	AgentServiceCACertFile = "/etc/ioi/service/pki/na-service-ca.crt"
	ServiceKeyFile         = "/etc/ioi/service/pki/service.key"
	ServiceCertFile        = "/etc/ioi/service/pki/service.crt"
	FQDN                   = "*.ioi-system.pod.cluster.local"
	ChanSize               = 500
)

// Agent
type Agent struct {
	engines        map[string]DataEngine
	ehs            []EventHandler
	CoreClient     *kubernetes.Clientset
	Clientset      *versioned.Clientset
	IOInfo         v1.NodeStaticIOInfo
	EnginesSwitch  int
	aggregatorConn *grpc.ClientConn
	serviceConn    *grpc.ClientConn
	mtls           bool
}

var (
	IoiClient         pb.IoiserviceClient
	IoiCtx            context.Context
	ClientHandlerChan = make(chan interface{}, ChanSize)
)

var node_agent = Agent{
	engines: make(map[string]DataEngine),
	ehs:     []EventHandler{},
}

func GetAgent() *Agent {
	return &node_agent
}

func CheckEngine(engine int) bool {
	if (GetAgent().EnginesSwitch & engine) > 0 {
		return true
	} else {
		return false
	}
}

func (c *Agent) RegisterEngine(engine DataEngine) {
	c.engines[engine.Type()] = engine
}

func (c *Agent) RegisterEventHandler(ch chan EventData, key string, f EventFunc, msg string, engine EventCacheEngine, needCache bool) {
	klog.Infof("Register Event Handler, key: %s, msg:%s", key, msg)
	found := false
	handle := EventHandle{
		engine:                   engine,
		needCacheBeforeExecution: needCache,
		key:                      key,
		f:                        f,
		msg:                      msg,
	}

	for i, eh := range c.ehs {
		if eh.ch == ch {
			c.ehs[i].handles = append(eh.handles, handle)
			found = true
			break
		}
	}

	if !found {
		c.ehs = append(c.ehs, EventHandler{
			ch:      ch,
			handles: []EventHandle{handle},
		})
	}
}

func StartServiceClient(tls bool) (*grpc.ClientConn, pb.IoiserviceClient, context.Context, error) {
	var creds credentials.TransportCredentials
	var err error

	addr := fmt.Sprintf("unix://%s", pb.IOISockPath)
	// force disable tls
	// tls = false
	if tls {
		creds, err = credentials.NewClientTLSFromFile(pb.TlsKeyDir+pb.CaCrt, "")
		if err != nil {
			klog.Warningf("Failed to create TLS credentials %v", err)
		}
	} else {
		creds = insecure.NewCredentials()
	}

	retryPolicy := `{
		"methodConfig": [{
		  "name": [
		    {"service": "ioiservice","method":"RegisterApp"},
			{"service": "ioiservice","method":"UnregisterApp"},
			{"service": "ioiservice","method":"SetAppLimit"},
			{"service": "ioiservice","method":"Configure"}
		  ],
		  "retryPolicy": {
			"MaxAttempts": 3,
			"InitialBackoff": "1s",
			"MaxBackoff": "3s",
			"BackoffMultiplier": 1.5,
			"RetryableStatusCodes": [ "UNAVAILABLE" ]
		  }
		}]}`
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds), grpc.WithDefaultServiceConfig(retryPolicy), grpc.WithConnectParams(grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  time.Second,
			Multiplier: 1.5,
			MaxDelay:   60 * time.Second,
		}}))
	if err != nil {
		klog.Warningf("Failed to connect to io server, error: %v", err)
		return conn, nil, nil, err
	} else {
		klog.Info("Successfully connect to the ioi-service")
	}

	ioi := pb.NewIoiserviceClient(conn)
	if ioi == nil {
		klog.Warningf("Failed to create ioi service client error: %v", err)
		return conn, nil, nil, err
	} else {
		klog.Info("Successfully create ioi service client")
	}

	ctx := context.Background()

	return conn, ioi, ctx, nil
}

func (c *Agent) Run() error {
	err := c.InitStaticCR()
	if err != nil {
		klog.Warning("init static CR fail, error: ", err)
		return err
	}

	klog.Info("2.1 static CR init successfully")
	c.serviceConn, IoiClient, IoiCtx, err = StartServiceClient(c.mtls)
	if err != nil {
		klog.Error("service client start failed.")
		return err
	}

	// initialize data engines
	for _, engine := range c.engines {
		err := engine.Initialize(c.CoreClient, c.Clientset, c.mtls)
		if err != nil {
			klog.Warningf("3.agent initialize engine failed, error:%v, type:%v", err, engine.Type())
		} else {
			klog.Info("3.agent initialize engine succeeded, engine:", engine.Type())
		}
	}

	// start agent server and client for aggregator
	go c.StartServer4Aggregator()
	nodeName := c.IOInfo.Spec.NodeName
	klog.Infof("4. StartClient4Aggregator node name %s", nodeName)
	err = c.StartClient4Aggregator(nodeName)
	if err != nil {
		return err
	}

	// Start event loop
	klog.Info("5. Start event loop")
	cases := make([]reflect.SelectCase, len(c.ehs))
	for i, eh := range c.ehs {
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(eh.ch),
		}
	}
	klog.Infof("5.1 event loop get cases: %v", cases)
	for {
		chosen, value, ok := reflect.Select(cases)
		if !ok {
			klog.Warning("select cases failed")
			break
		}

		data, ok := value.Interface().(EventData)
		if ok {
			klog.V(utils.INF).Infof("6. %s event data received", data.Type())
			for _, handle := range c.ehs[chosen].handles {
				if handle.key == data.Type() {
					var err error
					// Todo: run the handler in parallel
					if !handle.needCacheBeforeExecution || handle.engine == nil {
						err = handle.f(data)
					} else {
						err = handle.engine.HandleEvent(data, handle.f)
					}

					if err != nil {
						klog.Warning(handle.msg + ":" + err.Error())
					}
				}
			}
		} else {
			klog.Warning("event data is not valid")
		}
	}

	klog.Warning("7. event loop exit")
	return nil
}

func (c *Agent) Stop() error {
	// initialize data engines
	for _, engine := range c.engines {
		err := engine.Uninitialize()
		if err != nil {
			klog.Warning(err)
		}
	}

	if c.aggregatorConn != nil {
		c.aggregatorConn.Close()
	}

	if c.serviceConn != nil {
		c.serviceConn.Close()
	}

	return nil
}

func (c *Agent) InitStaticCR() error {
	// TODO: create CR
	NodeName := os.Getenv("Node_Name")
	c.IOInfo.APIVersion = "ioi.intel.com/v1"
	c.IOInfo.Kind = "NodeStaticIOInfo"
	c.IOInfo.Name = NodeName + "-nodestaticioinfo"
	c.IOInfo.Namespace = CrNamespace
	c.IOInfo.Spec.NodeName = NodeName
	PodIP := os.Getenv("Pod_IP")
	port := GetAggregatorPort()
	c.IOInfo.Spec.EndPoint = fmt.Sprintf("%s:%d", PodIP, port)
	klog.V(utils.INF).Info(" c.IOInfo.Spec.EndPoint: ", c.IOInfo.Spec.EndPoint)
	c.IOInfo.Spec.ResourceConfig = make(map[string]v1.ResourceConfigSpec)
	c.IOInfo.Status.DeviceStates = make(map[string]v1.DeviceState)

	return nil
}

func (c *Agent) InitClient() error {
	var err error

	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	c.CoreClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		klog.Warning("CoreClient init error: " + err.Error())
		return err
	}

	c.Clientset, err = versioned.NewForConfig(config)
	if err != nil {
		klog.Warning("Clientset init err")
		return err
	}

	return nil
}

func init() {
	klog.Info("0.Init agent client")
	c := GetAgent()
	c.mtls = false
	if c.mtls {
		err := utils.CertSetup(filepath.Join(utils.TlsPath, CAKeyFile), filepath.Join(utils.TlsPath, CACertFile), CommonName, []string{FQDN}, AgentKeyFile, AgentCertFile)
		if err != nil {
			klog.Errorf("cannot create node agent cert: %v", err)
		}
	}

	err := c.InitClient()
	if err != nil {
		klog.Warning("0.1 agent init client failed:", err)
		return
	} else {
		klog.Info("0.1 agent init client succeeded")
	}

	var node *v11.Node
	name := os.Getenv("Node_Name")
	var sleepTime int64 = 0
	var retryTimes int64 = 0
	var maxSleepTime int64 = 60
	for {
		node, err = c.CoreClient.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("0.2.2 get node failed %s", err)
			if sleepTime < maxSleepTime {
				sleepTime++
			}
			time.Sleep(time.Second * time.Duration(sleepTime))
			retryTimes++
			klog.Warningf("0.2.3 retry times %d time and get node failed: %s", retryTimes, err)
		} else {
			klog.Infof("0.2.1 node name %v and label %v", node.Name, node.Labels)
			break
		}
	}
	c.EnginesSwitch = 0
	var resources []string
	var csiEnable bool = true
	for k, v := range node.Labels {
		if k == IoiLabel {
			resources = strings.Split(v, "-")
		}
		if k == CsiSeperatedLabel {
			if v == "true" {
				csiEnable = false
			}
		}
	}
	for _, r := range resources {
		if r == "disk" || r == "all" {
			c.EnginesSwitch = (c.EnginesSwitch | DiskSwitch)
			if csiEnable {
				c.EnginesSwitch = (c.EnginesSwitch | CSISwitch)
			}
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
