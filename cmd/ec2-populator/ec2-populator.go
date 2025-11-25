package main

import (
	"context"
	"flag"
	"os"

	"github.com/kubev2v/forklift/cmd/ec2-populator/internal/config"
	"github.com/kubev2v/forklift/cmd/ec2-populator/internal/populator"
	"github.com/kubev2v/forklift/pkg/metrics"
	"k8s.io/klog/v2"
)

var version = "0.0.0"

func main() {
	klog.InitFlags(nil)

	cfg, showVer := parseFlags()

	if showVer {
		klog.Infof("ec2-populator version: %s", version)
		os.Exit(0)
	}

	if err := cfg.Validate(); err != nil {
		klog.Fatalf("Configuration validation failed: %v", err)
	}

	certsDirectory, err := os.MkdirTemp("", "certsdir")
	if err != nil {
		klog.Fatalf("Failed to create certs directory: %v", err)
	}

	metrics.StartPrometheusEndpoint(certsDirectory)

	if err := run(cfg); err != nil {
		klog.Fatalf("Population failed: %v", err)
	}

	klog.Info("EC2 volume population completed successfully")
}

func parseFlags() (*config.Config, bool) {
	cfg := &config.Config{}
	var showVer bool

	flag.StringVar(&cfg.Region, "region", "", "AWS region (required - where snapshot exists and volume will be created)")
	flag.StringVar(&cfg.TargetAvailabilityZone, "target-availability-zone", "", "AWS availability zone (required - where to create volume)")
	flag.StringVar(&cfg.SnapshotID, "snapshot-id", "", "EBS snapshot ID")
	flag.StringVar(&cfg.SecretName, "secret-name", "", "AWS credentials secret")
	flag.StringVar(&cfg.CRName, "cr-name", "", "Ec2VolumePopulator CR name")
	flag.StringVar(&cfg.CRNamespace, "cr-namespace", "", "CR namespace")
	flag.StringVar(&cfg.OwnerUID, "owner-uid", "", "PVC UID (for prime PVC identification)")
	flag.Int64Var(&cfg.PVCSize, "pvc-size", 0, "PVC size in bytes (passed by populator-machinery)")
	flag.BoolVar(&showVer, "version", false, "Show version")

	flag.Parse()

	return cfg, showVer
}

func run(cfg *config.Config) error {
	ctx := context.Background()

	pop, err := populator.New(cfg)
	if err != nil {
		return err
	}

	return pop.Run(ctx)
}
