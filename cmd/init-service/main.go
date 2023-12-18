/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/spf13/viper"
	v11 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
)

const (
	unitName string = "ioi-service.service"
)

func main() {
	ctx := context.Background()
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in-cluster config: %v", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create k8s client: %v", err)
	}
	// create systemd D-Bus connection
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to systemd D-Bus: %v", err)
	}
	defer conn.Close()
	resC := make(chan string)
	defer close(resC)
	// verify cert exist and valid
	certValid := checkCertValid()
	serviceRunning := checkServiceRunning(ctx, unitName, conn)
	if certValid && serviceRunning && !nodeLabelChanged(client) {
		klog.Info("cert valid, service running, return directly")
		return
	}
	// stop systemd unit
	if serviceRunning {
		_, err = conn.StopUnitContext(ctx, unitName, "replace", resC)
		if err != nil {
			log.Fatalf("Failed to stop unit %s: %v", unitName, err)
		}
		result := <-resC
		klog.Info("Stop unit result:", result)
	}
	if !certValid {
		err := generateTlsPairs()
		if err != nil {
			log.Fatalf("Failed to generate tls pairs: %v", err)
		}
		klog.Info("New Tls pair generated")
	}
	// check node label and set ioi-service.toml
	rdtEnabled, err := setNodeLabel(client)
	if err != nil {
		log.Fatalf("Failed to get node label: %v", err)
	}
	// flush service binary and configuration, start service
	moveServiceCfgBinary(rdtEnabled)
	// reload systemd daemon
	err = conn.ReloadContext(ctx)
	if err != nil {
		log.Fatalf("Failed to reload daemon: %v", err)
	}
	// start systemd unit
	_, err = conn.StartUnitContext(ctx, unitName, "replace", resC)
	if err != nil {
		log.Fatalf("Failed to start unit %s: %v", unitName, err)
	}
	result := <-resC

	if result != "done" {
		log.Fatalf("failed to start systemd unit %q, with result %q", unitName, result)
	}
	// started := checkServiceRunning(ctx, unitName, conn)
	// klog.Info("started:", started)
}

func getNodeLabel(client kubernetes.Interface) (string, error) {
	// get label value

	var node *v11.Node
	var err error
	name := os.Getenv("Node_Name")
	var sleepTime int64 = 0
	var retryTimes int64 = 0
	var maxSleepTime int64 = 60
	for {
		node, err = client.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("Init Service: 0.2 get node failed %s", err)
			if sleepTime < maxSleepTime {
				sleepTime++
			}
			time.Sleep(time.Second * time.Duration(sleepTime))
			retryTimes++
			klog.Warningf("Init Service:0.3 retry times %d time and get node failed: %s", retryTimes, err)
		} else {
			klog.Infof("Init Service: 0.1 node name %v and label %v", node.Name, node.Labels)
			break
		}
	}

	for k, v := range node.Labels {
		if k == "ioisolation" {
			return v, nil
		}
	}
	return "", fmt.Errorf("label ioisolation not found in current node")
}

func setNodeLabel(client kubernetes.Interface) (bool, error) {
	values, err := getNodeLabel(client)
	if err != nil {
		return false, err
	}
	rdtEnabled := false
	// check if rdt enabled
	resources := strings.Split(values, "-")
	for _, r := range resources {
		if r == "rdt" || r == "all" {
			rdtEnabled = true
			break
		}
	}
	// write label in ioi-service.toml
	viper.SetConfigFile("ioi-service.toml")
	viper.SetConfigType("toml")
	viper.AddConfigPath("./")
	if err := viper.ReadInConfig(); err != nil {
		return rdtEnabled, err
	}
	viper.Set("node_label", values)
	if err := viper.WriteConfig(); err != nil {
		return rdtEnabled, err
	}
	return rdtEnabled, nil
}

func nodeLabelChanged(client kubernetes.Interface) bool {
	new, err := getNodeLabel(client)
	if err != nil {
		klog.Errorf("Failed to get current node label: %v", err)
		return true
	}
	viper.SetConfigFile("/etc/ioi-service/ioi-service.toml")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/etc/ioi-service/")
	if err := viper.ReadInConfig(); err != nil {
		klog.Errorf("Failed to read ioi-service.toml: %v", err)
		return true
	}
	cur := viper.GetString("node_label")
	return !(new == cur)
}

func generateTlsPairs() error {
	// Generate CA private key
	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return fmt.Errorf("failed to generate CA private key: %v", err)
	}
	// Create CA certificate template
	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:      []string{"CN"},
			Locality:     []string{"Shanghai"},
			Organization: []string{"Intel"},
			CommonName:   "NARoot",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0), // Valid for 1 year
		SignatureAlgorithm:    x509.SHA384WithRSA,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	// Self-sign CA certificate
	caCertBytes, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %v", err)
	}
	// Write CA private key to file
	caPrivateKeyFile, err := os.Create(pb.TlsKeyDir + pb.CaKey)
	if err != nil {
		return fmt.Errorf("failed to create CA private key file: %v", err)
	}
	defer caPrivateKeyFile.Close()
	caPrivateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caPrivateKey)}
	pem.Encode(caPrivateKeyFile, caPrivateKeyPEM)

	// Write CA certificate to file
	caCertFile, err := os.Create(pb.TlsKeyDir + pb.CaCrt)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate file: %v", err)
	}
	defer caCertFile.Close()
	caCertPEM := &pem.Block{Type: "CERTIFICATE", Bytes: caCertBytes}
	pem.Encode(caCertFile, caCertPEM)
	caCert, err := x509.ParseCertificate(caCertPEM.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %v", err)
	}
	// Generate service private key
	servicePrivateKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return fmt.Errorf("failed to generate service private key: %v", err)
	}

	// Create service certificate template
	serviceTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Country:      []string{"CN"},
			Locality:     []string{"Shanghai"},
			Organization: []string{"Intel"},
			CommonName:   "ioiservice",
		},
		NotBefore:          time.Now(),
		NotAfter:           time.Now().AddDate(1, 0, 0), // Valid for 1 year
		SignatureAlgorithm: x509.SHA384WithRSA,
		DNSNames:           []string{"localhost"},
	}

	// Create service certificate
	serviceCertBytes, err := x509.CreateCertificate(rand.Reader, &serviceTemplate, caCert, &servicePrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to create service certificate: %v", err)
	}

	// Write service private key to file
	servicePrivateKeyFile, err := os.Create(pb.TlsKeyDir + pb.ServerKey)
	if err != nil {
		return fmt.Errorf("failed to create service private key file: %v", err)
	}
	defer servicePrivateKeyFile.Close()
	servicePrivateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(servicePrivateKey)}
	pem.Encode(servicePrivateKeyFile, servicePrivateKeyPEM)

	// Write service certificate to file
	serviceCertFile, err := os.Create(pb.TlsKeyDir + pb.ServerCrt)
	if err != nil {
		return fmt.Errorf("failed to create service certificate file: %v", err)
	}
	defer serviceCertFile.Close()
	serviceCertPEM := &pem.Block{Type: "CERTIFICATE", Bytes: serviceCertBytes}
	pem.Encode(serviceCertFile, serviceCertPEM)
	return nil
}

func checkServiceRunning(ctx context.Context, unitName string, conn *dbus.Conn) bool {
	sts, err := conn.ListUnitsByNamesContext(ctx, []string{unitName})
	if err != nil {
		klog.Errorf("Failed to list unit %s: %v", unitName, err)
		return false
	}
	for _, st := range sts {
		if st.Name == unitName {
			if st.ActiveState == "active" {
				return true
			}
		}
	}
	return false
}

func checkCertValid() bool {
	_, err := os.Stat(pb.TlsKeyDir + pb.ServerKey)
	if err != nil && os.IsNotExist(err) {
		klog.Error("Server key file not found")
		return false
	}
	// Load the CA certificate
	caCertPEM, err := os.ReadFile(pb.TlsKeyDir + pb.CaCrt)
	if err != nil {
		klog.Error("Failed to read CA certificate file:", err)
		return false
	}
	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		klog.Error("Failed to decode CA certificate PEM")
		return false
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		klog.Error("Failed to parse CA certificate:", err)
		return false
	}
	// Load the service certificate
	serviceCertPEM, err := os.ReadFile(pb.TlsKeyDir + pb.ServerCrt)
	if err != nil {
		klog.Error("Failed to read service certificate file:", err)
		return false
	}
	serviceCertBlock, _ := pem.Decode(serviceCertPEM)
	if serviceCertBlock == nil {
		klog.Error("Failed to decode service certificate PEM")
		return false
	}
	serviceCert, err := x509.ParseCertificate(serviceCertBlock.Bytes)
	if err != nil {
		klog.Error("Failed to parse service certificate:", err)
		return false
	}
	// Verify the service certificate against the CA certificate
	opts := x509.VerifyOptions{
		Roots:       x509.NewCertPool(),
		CurrentTime: serviceCert.NotBefore,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	opts.Roots.AddCert(caCert)
	if _, err := serviceCert.Verify(opts); err != nil {
		klog.Error("Certificate verification failed:", err)
		return false
	}
	return true
}

func moveServiceCfgBinary(rdtEnabled bool) {
	// move service binary and systemd configuration to host.
	cmd := exec.Command("mv", "./ioi-service", "/usr/local/bin/ioi-service")
	_, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to move ioi-service binary to /usr/local/bin: %v", err)
	}
	if rdtEnabled {
		cmd := exec.Command("mv", "./ioi-rdt-service.cfg", "/etc/ioi-service/ioi-rdt-service.cfg")
		_, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to move ioi-rdt-service.cfg to /etc/ioi-service: %v", err)
		}

		cmd = exec.Command("mv", "./fixedclass.yaml", "/etc/ioi-service/fixedclass.yaml")
		_, err = cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to move fixedclass.yaml to /etc/ioi-service: %v", err)
		}
	}
	cmd = exec.Command("mv", "./ioi-service.toml", "/etc/ioi-service/ioi-service.toml")
	_, err = cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to move ioi-service.toml to /etc/ioi-service: %v", err)
	}
	cmd = exec.Command("mv", "./ioi-service.service", "/usr/lib/systemd/system/ioi-service.service")
	_, err = cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to move ioi-service.service to /usr/lib/systemd/system: %v", err)
	}
}
