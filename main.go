package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"time"

	cosiapi "github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	cosistate "github.com/cosi-project/runtime/pkg/state/protobuf/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	cfgres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	v1alpha1res "github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

func main() {
	caCertPath := flag.String("ca-cert", "ca.crt", "OS root CA cert PEM file (from osrootsecret)")
	caKeyPath := flag.String("ca-key", "ca.key", "OS root CA private key PEM file (from osrootsecret)")
	listen := flag.String("listen", "0.0.0.0:50000", "listen address")
	nodeIP := flag.String("node-ip", "192.168.0.23", "node IP for SAN")
	hostname := flag.String("hostname", "ares-worker-4", "hostname for cert CN/SAN")
	cfgPath := flag.String("machine-config", "machine-config.yaml", "stub v1alpha1 machine config YAML")
	kubeletImage := flag.String("kubelet-image", "ghcr.io/siderolabs/kubelet:v1.32.13", "current kubelet image to advertise")
	talosVersion := flag.String("talos-version", "v1.11.6", "Talos version to advertise in MachineService.Version (used by upgrade-k8s compatibility check)")
	flag.Parse()

	tlsConf, err := buildTLSConfig(*caCertPath, *caKeyPath, *nodeIP, *hostname)
	if err != nil {
		log.Fatalf("tls config: %v", err)
	}

	st, err := buildState(*cfgPath, *kubeletImage)
	if err != nil {
		log.Fatalf("state: %v", err)
	}

	lis, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("listen %s: %v", *listen, err)
	}

	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConf)))

	cosiapi.RegisterStateServer(grpcServer, cosistate.NewState(st))
	machine.RegisterMachineServiceServer(grpcServer, &machineServer{talosVersion: *talosVersion})

	log.Printf("fake apid listening on %s (node=%s, hostname=%s)", *listen, *nodeIP, *hostname)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

type machineServer struct {
	machine.UnimplementedMachineServiceServer

	talosVersion string
}

func (s *machineServer) Version(_ context.Context, _ *emptypb.Empty) (*machine.VersionResponse, error) {
	return &machine.VersionResponse{
		Messages: []*machine.Version{
			{
				Metadata: &common.Metadata{},
				Version: &machine.VersionInfo{
					Tag:       s.talosVersion,
					Os:        "linux",
					Arch:      "arm64",
					GoVersion: "go1.24.6",
				},
				Platform: &machine.PlatformInfo{Name: "metal", Mode: "metal"},
			},
		},
	}, nil
}

func buildState(cfgPath, kubeletImage string) (state.State, error) {
	cfgBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("read machine config: %w", err)
	}

	provider, err := configloader.NewFromBytes(cfgBytes)
	if err != nil {
		return nil, fmt.Errorf("parse machine config: %w", err)
	}

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	ctx := context.Background()

	kubeletSpec := k8s.NewKubeletSpec(k8s.NamespaceName, "kubelet")
	kubeletSpec.TypedSpec().Image = kubeletImage
	if err := st.Create(ctx, kubeletSpec); err != nil {
		return nil, fmt.Errorf("seed kubelet spec: %w", err)
	}

	svc := v1alpha1res.NewService("kubelet")
	svc.TypedSpec().Running = true
	svc.TypedSpec().Healthy = true
	if err := st.Create(ctx, svc); err != nil {
		return nil, fmt.Errorf("seed kubelet service: %w", err)
	}

	mc := cfgres.NewMachineConfig(provider)
	if err := st.Create(ctx, mc); err != nil {
		return nil, fmt.Errorf("seed machine config: %w", err)
	}

	return st, nil
}

func buildTLSConfig(caCertPath, caKeyPath, nodeIP, hostname string) (*tls.Config, error) {
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}

	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read CA key: %w", err)
	}

	caCert, caKey, err := parseCA(caCertPEM, caKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse CA: %w", err)
	}

	serverCertPEM, serverKeyPEM, err := signServerCert(caCert, caKey, nodeIP, hostname)
	if err != nil {
		return nil, fmt.Errorf("sign server cert: %w", err)
	}

	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("load server keypair: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCertPEM) {
		return nil, fmt.Errorf("append CA cert to pool")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func parseCA(certPEM, keyPEM []byte) (*x509.Certificate, ed25519.PrivateKey, error) {
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("invalid CA cert PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("invalid CA key PEM")
	}

	key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("unsupported CA key type %T (need ed25519)", key)
	}

	return cert, edKey, nil
}

func signServerCert(ca *x509.Certificate, caKey ed25519.PrivateKey, nodeIP, hostname string) ([]byte, []byte, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: hostname},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{hostname},
		IPAddresses:  []net.IP{net.ParseIP(nodeIP)},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, pub, caKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}
