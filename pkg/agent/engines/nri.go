/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package engines

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	"sigs.k8s.io/IOIsolation/pkg/agent/oob"
)

const (
	annMockDisk      = "blockio.kubernetes.io/mock.stress"
	annMockNet       = "blockio.kubernetes.io/mock.speed"
	annRDT           = "rdt.resources.beta.kubernetes.io/pod"
	annRDTQuantity   = "rdt.quantity.resources.beta.kubernetes.io/resources"
	NriPluginName    = "ioi.v1"
	usage            = `IOIsolation - Disk/Network/RDT/RDTQuantity IO aware scheduler and control`
	DefaultConfigDir = "/etc/ioi/config"
	conSuffix        = ".scope"
	podPrefix        = "/sys/fs/cgroup"
	conPrefix        = "/cri-containerd-"
)

var (
	cfg     config
	_       = stub.ConfigureInterface(&IOPlugin{})
	TypeNRI = "NRI"
	mutex   sync.Mutex
)

var BkEngine *oob.OobServer

// NRIConfig
type NRIConfig struct {
	Nri     bool                 `toml:"nri"`
	Plugins map[string]toml.Tree `toml:"plugins"`
}

type BlockIOPluginConfig struct {
	Enable bool   `toml:"enable"`
	Idx    string `toml:"idx"`
}

type NetworkIOPluginConfig struct {
	Enable           bool   `toml:"enable"`
	Idx              string `toml:"idx"`
	QosClassFilePath string `toml:"qos"`
}

func LoadNRIConfig(path string) (*NRIConfig, error) {
	config, err := loadNRIConfigFile(path)
	if err != nil {
		klog.Errorf("failed to parse config: %w", err)
		return nil, err
	}
	return config, nil
}

func loadNRIConfigFile(path string) (*NRIConfig, error) {
	config := &NRIConfig{}
	file, err := toml.LoadFile(path)

	if err != nil {
		return nil, fmt.Errorf("failed to load TOML: %s: %w", path, err)
	}

	if err := file.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal TOML: %w", err)
	}
	return config, nil
}

// IOPlugin
type config struct {
	LogFile string   `json:"logFile"`
	Events  []string `json:"events"`
}

type IOPlugin struct {
	PluginName string
	PluginIdx  string
	Events     string
	LogFile    string
	Stub       stub.Stub
	Mask       stub.EventMask
	Engine     *NriEngine
}

func (p *IOPlugin) Configure(config, runtime, version string) (stub.EventMask, error) {
	// klog.Infof("got configuration data: %q from runtime %s %s", config, runtime, version)
	if config == "" {
		return p.Mask, nil
	}

	oldCfg := cfg
	err := yaml.Unmarshal([]byte(config), &cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to parse provided configuration: %w", err)
	}

	p.Mask, err = api.ParseEventMask(cfg.Events...)
	if err != nil {
		return 0, fmt.Errorf("failed to parse events in configuration: %w", err)
	}

	if cfg.LogFile != oldCfg.LogFile {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			klog.Errorf("failed to open log file %q: %v", cfg.LogFile, err)
			return 0, fmt.Errorf("failed to open log file %q: %w", cfg.LogFile, err)
		}
		klog.SetOutput(f)
	}

	return p.Mask, nil
}

func (p *IOPlugin) Synchronize(pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	if agent.CheckEngine(agent.DiskSwitch) || agent.CheckEngine(agent.NetSwitch) {
		dropRepeat := make(map[string]int)
		for _, pod := range pods {
			if _, ok := dropRepeat[pod.Uid]; ok {
				continue
			}
			dropRepeat[pod.Uid] = 1
			p.Engine.AnalysisNriData(pod, utils.RunPod)
		}
	} else {
		klog.V(utils.DBG).Info("Disk or net engine does not start")
	}

	if agent.CheckEngine(agent.RDTSwitch) || agent.CheckEngine(agent.RDTQuantitySwitch) {
		dropRepeatc := make(map[string]int)
		for _, container := range containers {
			if _, ok := dropRepeatc[container.Id]; ok {
				continue
			}
			dropRepeatc[container.Id] = 1
			for _, pod := range pods {
				if pod.Id == container.PodSandboxId {
					p.Engine.AnalysisNriContainerData(pod, container, utils.StartContainer)
					break
				}
			}
		}
	} else {
		klog.V(utils.DBG).Info("RDT or RDT Quantity engine does not start")
	}

	return nil, nil
}

func (p *IOPlugin) Shutdown() {
	dump("Shutdown")
}

func (p *IOPlugin) RunPodSandbox(pod *api.PodSandbox) error {
	klog.V(utils.DBG).Info("RunPodSandbox,podName: ", pod.Name)
	if agent.CheckEngine(agent.DiskSwitch) || agent.CheckEngine(agent.NetSwitch) {
		p.Engine.AnalysisNriData(pod, utils.RunPod)
	} else {
		klog.V(utils.DBG).Info("Disk or net engine does not start")
	}

	return nil
}

func (p *IOPlugin) StopPodSandbox(pod *api.PodSandbox) error {
	klog.V(utils.DBG).Info("StopPodSandbox,podName: ", pod.Name)
	if agent.CheckEngine(agent.DiskSwitch) || agent.CheckEngine(agent.NetSwitch) {
		p.Engine.AnalysisNriData(pod, utils.StopPod)
	} else {
		klog.V(utils.DBG).Info("Disk or net engine does not start")
	}
	return nil
}

func (p *IOPlugin) RemovePodSandbox(pod *api.PodSandbox) error {
	return nil
}

func (p *IOPlugin) CreateContainer(pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	adjust := &api.ContainerAdjustment{}
	return adjust, nil, nil
}

func (p *IOPlugin) PostCreateContainer(pod *api.PodSandbox, container *api.Container) error {
	return nil
}

func (p *IOPlugin) StartContainer(pod *api.PodSandbox, container *api.Container) error {
	if agent.CheckEngine(agent.RDTSwitch) || agent.CheckEngine(agent.RDTQuantitySwitch) {
		p.Engine.AnalysisNriContainerData(pod, container, utils.StartContainer)
	} else {
		klog.V(utils.DBG).Info("RDT or RDT Quantity engine does not start")
	}
	return nil
}

func (p *IOPlugin) PostStartContainer(pod *api.PodSandbox, container *api.Container) error {
	return nil
}

func (p *IOPlugin) UpdateContainer(pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	return nil, nil
}

func (p *IOPlugin) PostUpdateContainer(pod *api.PodSandbox, container *api.Container) error {
	return nil
}

func (p *IOPlugin) StopContainer(pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	if agent.CheckEngine(agent.DiskSwitch) || agent.CheckEngine(agent.NetSwitch) {
		p.Engine.AnalysisNriData(pod, utils.StopContainer)
	} else {
		klog.V(utils.DBG).Info("Disk or net engine does not start")
	}

	if agent.CheckEngine(agent.RDTSwitch) || agent.CheckEngine(agent.RDTQuantitySwitch) {
		p.Engine.AnalysisNriContainerData(pod, container, utils.StopContainer)
	} else {
		klog.V(utils.DBG).Info("RDT or RDT Quantity engine does not start")
	}
	return nil, nil
}

func (p *IOPlugin) RemoveContainer(pod *api.PodSandbox, container *api.Container) error {
	return nil
}

func (p *IOPlugin) onClose() {
	klog.Error("Fatal Error: Connection to the runtime lost, exiting...")
	// os.Exit(0)
}

// Dump one or more objects, with an optional global prefix and per-object tags.
func dump(args ...interface{}) {
	var (
		prefix string
		idx    int
	)

	if len(args)&0x1 == 1 {
		prefix = args[0].(string)
		idx++
	}

	for ; idx < len(args)-1; idx += 2 {
		tag, obj := args[idx], args[idx+1]
		msg, err := yaml.Marshal(obj)
		if err != nil {
			klog.Warningf("%s: %s: failed to dump object: %v", prefix, tag, err)
			continue
		}

		if prefix != "" {
			klog.Infof("%s: %s:", prefix, tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				klog.V(utils.DBG).Infof("%s:    %s", prefix, line)
			}
		} else {
			klog.V(utils.DBG).Infof("%s:", tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				klog.V(utils.DBG).Infof("  %s", line)
			}
		}
	}
}

func (p *IOPlugin) Run() {
	var (
		opts []stub.Option
		err  error
	)

	if p.LogFile != "" {
		cfg.LogFile = p.LogFile
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			klog.Fatalf("failed to open log file %q: %v", cfg.LogFile, err)
		}
		mutex.Lock()
		klog.SetOutput(f)
		mutex.Unlock()
	}

	if p.PluginName != "" {
		opts = append(opts, stub.WithPluginName(p.PluginName))
	}
	if p.PluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(p.PluginIdx))
	}

	if p.Stub, err = stub.New(p, append(opts, stub.WithOnClose(p.onClose))...); err != nil {
		klog.Fatalf("failed to create plugin stub: %v", err)
	}

	err = p.Stub.Run(context.Background())
	if err != nil {
		klog.Errorf("nri plugin exited with error %v", err)
		startOOBEngine(err)
	}
}

func startOOBEngine(reason error) {
	// launch nri engine failed, and start oob engine as a backup
	var err error
	klog.V(utils.INF).Info("initializing the nri engine failed, and start oob engine. Failed reason is: ", reason)
	BkEngine, err := oob.NewOobServer("/opt/ioi/cgroup/")
	if err != nil {
		klog.Errorf("New OOB server error: %v", err)
	}
	err = BkEngine.Initialize()
	if err != nil {
		klog.Warning("oob engine failed, error:", err)
	} else {
		klog.V(utils.INF).Info("oob engine succeeded")
	}
}

// NriEngine
type NriEngine struct {
	config string
}

func (e *NriEngine) Type() string {
	return TypeNRI
}

func (e *NriEngine) Initialize(coreClient *kubernetes.Clientset, client *versioned.Clientset, mtls bool) error {
	klog.Info("3.3 initializing the nri engine")
	e.config = filepath.Join(DefaultConfigDir, "config.toml")
	return e.runNRIApp()
}

func (e *NriEngine) Uninitialize() error {
	return nil
}

func (e *NriEngine) runNRIApp() error {
	configPath := e.config
	_, err := os.Stat(configPath)
	if err != nil {
		startOOBEngine(err)
		return err
	}
	var config *NRIConfig
	if !os.IsNotExist(err) || configPath != "" {
		config, err = LoadNRIConfig(configPath)
		if err != nil || config == nil {
			startOOBEngine(err)
			return err
		}
	}
	e.startPlugin(config)
	return nil

}

func (e *NriEngine) startPlugin(config *NRIConfig) {
	// TODO: Dynamic detect which runtime type is and choose to enable/disable NRI
	for key, data := range config.Plugins {
		if key == NriPluginName {
			blockIOConfig := &BlockIOPluginConfig{}
			if err := data.Unmarshal(blockIOConfig); err != nil {
				klog.Errorf("Unmarshal blockIO config error %v", err)
				continue
			}
			if blockIOConfig.Enable {
				plugin := &IOPlugin{
					PluginName: NriPluginName,
					PluginIdx:  blockIOConfig.Idx,
					Engine:     e,
				}
				go plugin.Run()
			}
		}
	}
}

func (e *NriEngine) extractAnnotation(anns map[string]string, mock string) string {
	if val, ok := anns[mock]; ok {
		return val
	} else {
		return ""
	}
}

func (e *NriEngine) AnalysisNriData(pod *api.PodSandbox, op int) {
	klog.V(utils.INF).Infof("podId: %s, CGroupPath:%s", pod.Name, pod.Linux.CgroupParent)
	prf := agent.PodRunInfo{
		Operation:      op,
		PodName:        pod.Name,
		Namespace:      pod.Namespace,
		NetNamespace:   pod.Linux.Namespaces, // fixme: need to check later
		CGroupPath:     pod.Linux.CgroupParent,
		DiskAnnotation: e.extractAnnotation(pod.Annotations, utils.BlockIOConfigAnno),
	}
	pif := agent.PodEventInfo{
		pod.Uid: prf,
	}

	PodInfos := make(map[string]agent.PodEventInfo)
	PodInfos["dev"] = pif
	pData := agent.PodData{
		T:         0,
		PodEvents: PodInfos,
	}
	if len(agent.PodInfoChan) == cap(agent.PodInfoChan) {
		klog.Error("PodInfoChan is full")
		return
	}
	agent.PodInfoChan <- &pData
}

func (e *NriEngine) AnalysisNriContainerData(pod *api.PodSandbox, container *api.Container, op int) {
	containerCgroupPath := podPrefix + pod.Linux.CgroupParent + conPrefix + container.Id + conSuffix
	cif := agent.ContainerInfo{
		Operation:             op,
		PodName:               pod.Name,
		PodUid:                pod.Uid,
		ContainerName:         container.Name,
		ContainerId:           container.Id,
		Namespace:             pod.Namespace,
		ContainerCGroupPath:   containerCgroupPath,
		RDTAnnotation:         e.extractAnnotation(pod.Annotations, annRDT),
		RDTQuantityAnnotation: e.extractAnnotation(pod.Annotations, annRDTQuantity),
	}
	ContainerInfoes := make(map[string]agent.ContainerInfo)
	ContainerInfoes[container.Id] = cif
	cdata := agent.ContainerData{
		T:              0,
		ContainerInfos: ContainerInfoes,
	}
	if len(agent.ContainerInfoChan) == cap(agent.ContainerInfoChan) {
		klog.Error("ContainerInfoChan is full")
		return
	}
	agent.ContainerInfoChan <- &cdata
}

func IsPodRuning(podId string) bool {
	if BkEngine == nil {
		return true
	} else {
		return BkEngine.IsPodRuning(podId)
	}
}

func init() {
	engine := &NriEngine{}
	agent.GetAgent().RegisterEngine(engine)
}
