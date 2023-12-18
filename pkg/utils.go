/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"hash/fnv"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc/credentials"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	hashutil "k8s.io/kubernetes/pkg/util/hash"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
)

const (
	RDTConfigAnno         string  = "rdt.resources.beta.kubernetes.io/pod"
	RDTQuantityConfigAnno string  = "rdt.quantity.resources.beta.kubernetes.io/resources"
	BlockIOConfigAnno     string  = "blockio.kubernetes.io/resources"
	NetworkIOConfigAnno   string  = "networkio.kubernetes.io/resources"
	GroupName             string  = "ioi.intel.com"
	CrNamespace           string  = "ioi-system"
	APIVersion            string  = "ioi.intel.com/v1"
	RBPS                  string  = "rbps"
	WBPS                  string  = "wbps"
	RIOPS                 string  = "riops"
	WIOPS                 string  = "wiops"
	Ingress               string  = "ingress"
	Egress                string  = "egress"
	IoiNamespace          string  = "ioi-system"
	KubesystemNamespace   string  = "kube-system"
	MB                    float64 = (1000 * 1000)
	MiB                   float64 = (1024 * 1024)
	// IoiNodeTag          string = "ioisolation"
	GA_Type              string  = "GA"
	BE_Type              string  = "BE"
	BT_Type              string  = "BT"
	BE_Min_BW            string  = "BE_MIN"
	AVAIL_STORAGE        string  = "AVAIL_STORAGE"
	MAX_BT_LIMIT         float64 = 1 << 20
	TlsPath              string  = "/etc/ioi/control/pki"
	DeviceIDConfigMap    string  = "volume-device-map"
	AnnoDeviceID         string  = "ioi.intel.com/selected-device-id"
	AnnoVolPath          string  = "ioi.intel.com/selected-vol-path"
	StorageProvisioner   string  = "localstorage.csi.k8s.io"
	AllocatedIOAnno      string  = "ioi.intel.com/allocated-io"
	NodeStatusInfoSuffix string  = "-nodeiostatus"
	NodeStaticInfoSuffix string  = "-nodestaticioinfo"
	NodeIOStatusCR       string  = "NodeIOStatus"
	LOGLEVEL             string  = "LOGLEVEL"
	INTERVAL             string  = "INTERVAL"
	SYSTEM               string  = "SYSTEM"
	AdminName            string  = "admin-configmap"
	ProcCPUInfoName      string  = "cpuinfo"
	ProcRootDir          string  = "/proc"
	IntelVendorID        string  = "GenuineIntel"
	DeviceInUse          string  = "DeviceInUse"
)
const (
	GaIndex int32 = iota
	BtIndex
	BeIndex
	GroupIndex
)

const (
	StopPod        int = 0b1
	RunPod         int = 0b10
	StopContainer  int = 0b100
	StartContainer int = 0b1000
)

const (
	NotEnabled string = "NotEnabled"
	Profiling  string = "Profiling"
	Ready      string = "Ready"
	Fail       string = "Fail"
)

const (
	EmptyDir    string = "emptyDir"
	NonEmptyDir string = "non-emptyDir"
)

const (
	CommonIOIType      int32 = 0
	DiskIOIType        int32 = 1
	NetIOIType         int32 = 2
	RDTIOIType         int32 = 3
	RDTQuantityIOIType int32 = 4
)

// State machine: NotEnabled -> Profiling -> Ready

var IndexToType = map[int32]string{
	GaIndex: GA_Type,
	BtIndex: BT_Type,
	BeIndex: BE_Type,
}

var TypeToIndex = map[string]int32{
	GA_Type: GaIndex,
	BT_Type: BtIndex,
	BE_Type: BeIndex,
}

var (
	INF klog.Level = 1
	DBG klog.Level = 2
)

type MappingRatio struct {
	BlockSize resource.Quantity
	Ratio     float64
}

type Workload struct {
	InBps    float64
	OutBps   float64
	InIops   float64
	OutIops  float64
	TotalBps float64
}

//type DiskInfoBs map[string]*Workload // blocksize: workload

type DiskInfo struct {
	Name         string             `json:"name,omitempty"`
	MajorMinor   string             `json:"maj_min,omitempty"`
	Type         string             `json:"type,omitempty"`
	MountPoint   string             `json:"mountpoint,omitempty"`
	Capacity     string             `json:"capacity,omitempty"`
	TotalBPS     float64            `json:"total_bps,omitempty"`
	TotalRBPS    float64            `json:"total_rbps,omitempty"`
	TotalWBPS    float64            `json:"total_wbps,omitempty"`
	ReadRatio    map[string]float64 `json:"read_ratio,omitempty"` // blocksize: ratio
	WriteRatio   map[string]float64 `json:"write_ratio,omitempty"`
	ReadMpRatio  []MappingRatio     `json:"readmpratio,omitempty"`
	WriteMpRatio []MappingRatio     `json:"writempratio,omitempty"`
}

type DiskInfos map[string]DiskInfo //diskid

type NicInfo struct {
	Name          string  `json:"name,omitempty"`
	Macaddr       string  `json:"macaddr,omitempty"`
	IpAddr        string  `json:"ipaddr,omitempty"`
	Type          string  `json:"type,omitempty"`
	TotalBPS      float64 `json:"total_bps,omitempty"`
	SndGroupBPS   float64 `json:"snd_group_bps,omitempty"`
	RcvGroupBPS   float64 `json:"rcv_group_bps,omitempty"`
	QueueNum      int     `json:"queue_num,omitempty"`
	GroupSettable bool    `json:"group_setable,omitempty"`
}

type NicInfos map[string]NicInfo

type IOPoolStatus struct {
	Total         float64
	In            float64
	Out           float64
	UnderPressure bool
	Num           int
}

func (in *IOPoolStatus) DeepCopy() *IOPoolStatus {
	if in == nil {
		return nil
	}
	out := &IOPoolStatus{}
	*out = *in
	return out
}

type PVCInfo struct {
	DevID      string
	ReqStorage int64
	Reuse      bool
}

type IOResourceRequest struct {
	IORequest      *pb.IOResourceRequest
	StorageRequest int64
}

type SpecialBool struct {
	Bool bool
}

type LogLevelConfigInfo struct {
	Info  bool `json:"info"`
	Debug bool `json:"debug"`
}

type IntervalConfigInfo struct {
	Interval int `json:"interval"`
}

type RDTClassInfo struct {
	L3CacheReq float64 `json:"l3cacherequest,omitempty"`
	L3CacheLim float64 `json:"l3cachelimit,omitempty"`
	MbaReq     float64 `json:"mbarequest,omitempty"`
	MbaLim     float64 `json:"mbalimit,omitempty"`
}

type RDTAppInfo struct {
	ContainerName string       `json:"containername"`
	CgroupPath    string       `json:"cgrouppath"`
	RDTClass      string       `json:"rdtclass"`
	PodUid        string       `json:"poduid"`
	PodName       string       `json:"podname"`
	RDTClassInfo  RDTClassInfo `json:"rdtclassinfo,omitempty"`
}

type DiskCgroupInfo struct {
	CgroupPath string
	DeviceId   []string
}

type AllocatedIO map[int64]map[string]DeviceAllocatedIO // Resource type(1 disk, 2 net) -> Device Name-> Device Resource
type DeviceAllocatedIO struct {
	QoS        int32    `json:"qos"`
	BlockSize  string   `json:"blocksize,omitempty"`
	Request    Quantity `json:"request,omitempty"`
	Limit      Quantity `json:"limit,omitempty"`
	RawRequest Quantity `json:"rawRequest,omitempty"`
	RawLimit   Quantity `json:"rawLimit,omitempty"`
}

type Quantity struct {
	Inbps   float64 `json:"inbps,omitempty"`
	Outbps  float64 `json:"outbps,omitempty"`
	Iniops  float64 `json:"iniops,omitempty"`
	Outiops float64 `json:"outiops,omitempty"`
}

func IsIOIProvisioner(client kubernetes.Interface, pvc, ns string) (bool, error) {
	claim, err := client.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), pvc, metav1.GetOptions{})
	if err != nil {
		klog.V(INF).Infof("Get PVC error: %v", err)
		return false, err
	}
	sc, err := client.StorageV1().StorageClasses().Get(context.TODO(), *claim.Spec.StorageClassName, metav1.GetOptions{})
	if err != nil || sc.Provisioner != StorageProvisioner {
		klog.V(INF).Infof("Get storage class %v: %v", *claim.Spec.StorageClassName, err)
		return false, err
	}
	return true, nil
}

type BlockDevice struct {
	Name      string        `json:"name,omitempty"`
	Type      string        `json:"type,omitempty"`
	PName     string        `json:"pkname,omitempty"`
	MajMin    string        `json:"maj:min,omitempty"`
	AvailSize int64         `json:"availsize,omitempty"`
	Mount     string        `json:"mountpoint,omitempty"`
	Model     string        `json:"model,omitempty"`
	Serial    string        `json:"serial,omitempty"`
	Rota      SpecialBool   `json:"rota,omitempty"`
	Children  []BlockDevice `json:"children,omitempty"`
}
type PodAnnoThroughput map[string]VolumeAnnoThroughput

type VolumeAnnoThroughput struct {
	Requests BlockIOConfig `json:"requests,omitempty"`
	Limits   BlockIOConfig `json:"limits,omitempty"`
}

type BlockIOConfig struct {
	Rbps string `json:"rbps,omitempty"`
	Wbps string `json:"wbps,omitempty"`
}

type PodAnnotationIO struct {
	Request WRSum
	Limit   WRSum
}
type WRSum struct {
	Read  IOSum
	Write IOSum
}
type IOSum struct {
	Bps  float64
	IOps int64
}

func FindMappingRatio(bs resource.Quantity, ratios []MappingRatio) (resource.Quantity, float64) {
	for idx, ratio := range ratios {
		if bs.Equal(ratio.BlockSize) {
			return ratio.BlockSize, ratio.Ratio
		} else if bs.Cmp(ratio.BlockSize) == -1 {
			if idx == 0 {
				return ratio.BlockSize, ratio.Ratio
			} else {
				return ratios[idx-1].BlockSize, ratios[idx-1].Ratio
			}
		}
	}
	if len(ratios) > 0 {
		return ratios[len(ratios)-1].BlockSize, ratios[len(ratios)-1].Ratio
	}
	return resource.MustParse("32k"), 1
}
func ParseDiskRequestBandwidth(config string) (resource.Quantity, resource.Quantity, error) {
	if config == "" {
		return resource.Quantity{}, resource.Quantity{}, fmt.Errorf("disk request value is empty")
	}
	BwStr, BsStr, found := strings.Cut(config, "/")
	if len(BsStr) > 0 && (BsStr[len(BsStr)-1] == 'B') {
		BsStr = BsStr[:len(BsStr)-1]
	}
	var err error
	var bw, bs resource.Quantity
	if found {
		bw, err = resource.ParseQuantity(BwStr)
		if err != nil {
			return resource.Quantity{}, resource.Quantity{}, err
		}
		bs, err = resource.ParseQuantity(BsStr)
		if err != nil {
			return resource.Quantity{}, resource.Quantity{}, err
		}
	} else {
		bw, err = resource.ParseQuantity(BwStr)
		if err != nil {
			return resource.Quantity{}, resource.Quantity{}, err
		}
		bs, _ = resource.ParseQuantity("4k")
	}
	return bw, bs, nil
}

// HashWithoutState remove the state field then compare
func HashObject(obj interface{}) uint64 {
	if obj == nil {
		return 0
	}
	hash := fnv.New32a()

	hashutil.DeepHashObject(hash, obj)
	return uint64(hash.Sum32())
}

func ParseMappingRatio(ratio map[string]float64) []MappingRatio {
	ratios := []MappingRatio{}
	for q, r := range ratio {
		bs, err := resource.ParseQuantity(q)
		if err != nil || r <= 0 {
			klog.Error("Parse Mapping ratio error: ", err)
			continue
		}
		ratios = append(ratios, MappingRatio{bs, r})
	}
	sort.Slice(ratios, func(i, j int) bool {
		return ratios[i].BlockSize.Cmp(ratios[j].BlockSize) == -1
	})
	return ratios
}
func (b *SpecialBool) UnmarshalJSON(data []byte) error {
	sData := string(data)
	if strings.Contains(sData, `true`) || strings.Contains(sData, `1`) || strings.Contains(sData, `"true"`) || strings.Contains(sData, `"1"`) {
		b.Bool = true
	} else if strings.Contains(sData, `false`) || strings.Contains(sData, `0`) || strings.Contains(sData, `"false"`) || strings.Contains(sData, `"0"`) {
		b.Bool = false
	} else {
		klog.Errorf("SpecialBool %s unmarshal error", sData)
		b.Bool = false
	}
	return nil
}

func (b *SpecialBool) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.Bool)
}
func CertSetup(rootKeyPath, rootCertPath, cn string, dns []string, dstKeyPath, dstCertPath string) (err error) {
	caKeyFile := rootKeyPath
	caCertFile := rootCertPath
	caKeyPEM, err := os.ReadFile(caKeyFile)
	if err != nil {
		return fmt.Errorf("failed to read CA private key: %v", err)
	}
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	caKey, err := x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		caKey, err = x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse CA private key: %v", err)
		}
	} else {
		pk, ok := caKey.(*rsa.PrivateKey)
		if !ok {
			return fmt.Errorf("failed to parse CA private key: %v", err)
		}
		caKey = pk
	}

	// load root CA
	caCertPEM, err := os.ReadFile(caCertFile)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %v", err)
	}
	caCertBlock, _ := pem.Decode(caCertPEM)
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %v", err)
	}
	// create cert configuration and set up our server certificate
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %v", err)
	}
	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Intel"},
			Country:      []string{"CN"},
			Locality:     []string{"Shanghai"},
			CommonName:   cn,
		},
		NotBefore:          time.Now().AddDate(0, 0, -1),
		NotAfter:           time.Now().AddDate(1, 0, 0),
		SignatureAlgorithm: x509.SHA384WithRSA,
		DNSNames:           dns,
	}
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, caCert, &certPrivKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	// save generated key
	keyOut, err := os.Create(dstKeyPath)
	if err != nil {
		return fmt.Errorf("failed to create key file: %v", err)
	}
	defer keyOut.Close()
	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey)})
	if err != nil {
		return fmt.Errorf("failed to encode key: %v", err)
	}
	// save generated crt
	certOut, err := os.Create(dstCertPath)
	if err != nil {
		return fmt.Errorf("failed to create certificate file: %v", err)
	}
	defer certOut.Close()
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if err != nil {
		return fmt.Errorf("failed to encode certificate: %v", err)
	}
	klog.Info("Key and certificate files generated successfully.")
	return
}

func LoadKeyPair(caPath, keyPath, certPath string, server bool) credentials.TransportCredentials {
	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		klog.Errorf("failed to load server certification: %v", err)
		os.Exit(1)
	}

	data, err := os.ReadFile(caPath)
	if err != nil {
		klog.Errorf("failed to load CA file: %v", err)
		os.Exit(1)
	}

	capool := x509.NewCertPool()
	if !capool.AppendCertsFromPEM(data) {
		klog.Error("can't add ca cert")
		os.Exit(1)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}
	if server {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConfig.ClientCAs = capool
	} else {
		tlsConfig.RootCAs = capool
	}
	return credentials.NewTLS(tlsConfig)
}

func RunBash(v1 string) (string, error) {
	var res bytes.Buffer
	var cmd *exec.Cmd
	fields := strings.Fields(v1)
	if len(fields) > 1 {
		cmd = exec.Command(fields[0], fields[1:]...)
	} else {
		cmd = exec.Command(fields[0])
	}

	cmd.Stdout = &res
	err := cmd.Run()
	if err != nil {
		klog.Errorf("run bash %s failed due to %v", v1, err)
		return "", err
	}
	return res.String(), nil
}

func SetLogLevel(info bool, debug bool) {

	if info {
		INF = 1
	} else {
		INF = 3
	}
	if debug {
		DBG = 2
	} else {
		DBG = 4
	}
}

func Contains(arr []string, num string) bool {
	for _, value := range arr {
		if value == num {
			return true
		}
	}
	return false
}

// GetVendorIDByCPUInfo returns vendor_id like AuthenticAMD from cpu info, e.g.
// vendor_id       : AuthenticAMD
// vendor_id       : GenuineIntel
func GetVendorIDByCPUInfo(path string) (string, error) {
	vendorID := "unknown"
	f, err := os.Open(path)
	if err != nil {
		return vendorID, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if err := s.Err(); err != nil {
			return vendorID, err
		}

		line := s.Text()

		// get "vendor_id" from first line
		if strings.Contains(line, "vendor_id") {
			attrs := strings.Split(line, ":")
			if len(attrs) >= 2 {
				vendorID = strings.TrimSpace(attrs[1])
				break
			}
		}
	}
	return vendorID, nil
}

func GetCPUInfoPath() string {
	return filepath.Join(ProcRootDir, ProcCPUInfoName)
}

// cat /proc/cpuinfo | grep vendor_id
func IsIntelPlatform() bool {
	vendor, err := GetVendorIDByCPUInfo(GetCPUInfoPath())
	if err != nil || vendor != IntelVendorID {
		return false
	} else {
		return true
	}
}

func TransPVCInfoMapToDevice(in map[string]*PVCInfo) map[string]string {
	out := make(map[string]string)
	for k, v := range in {
		out[k] = v.DevID
	}
	return out
}

func MergePVCInfo(current, added map[string]*PVCInfo) map[string]*PVCInfo {
	for k, v := range added {
		current[k] = v
	}
	return current
}

func GetDiskId(model string, serial string) string {
	if model == "" && serial == "" {
		return ""
	}
	if model == "" {
		return serial
	}
	if serial == "" {
		return model
	}
	return fmt.Sprintf("%s_%s", model[:3], serial)
}

func StringsToJson(s []string) (string, error) {
	jsonBytes, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

func JsonToStrings(jsonStr string) ([]string, error) {
	var s []string
	err := json.Unmarshal([]byte(jsonStr), &s)
	if err != nil {
		return nil, err
	}
	return s, nil
}
