package collector

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	ec2client "github.com/kubev2v/forklift/pkg/provider/ec2/inventory/client"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/ec2/testutil"
	sharedtestutil "github.com/kubev2v/forklift/pkg/provider/testutil"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("EC2 Collector", func() {
	var (
		fakeEC2   *testutil.FakeEC2API
		db        libmodel.DB
		collector *Collector
		provider  *api.Provider
		dbPath    string
	)

	BeforeEach(func() {
		// Create fake EC2 API
		fakeEC2 = testutil.NewFakeEC2API()

		// Create provider
		provider = sharedtestutil.NewProviderBuilder().
			WithName("test-provider").
			WithNamespace("test").
			WithType(api.EC2).
			Build()

		// Create temp DB file
		tmpDir, err := os.MkdirTemp("", "ec2-collector-test")
		Expect(err).NotTo(HaveOccurred())
		dbPath = filepath.Join(tmpDir, "test.db")

		// Create DB with EC2 models
		db = libmodel.New(dbPath, model.All()...)
		err = db.Open(true)
		Expect(err).NotTo(HaveOccurred())

		// Create client with fake EC2 API
		client := ec2client.NewWithClient(fakeEC2, "us-east-1")

		// Create collector
		secret := testutil.NewEC2Secret("test-secret", "test", "us-east-1")
		c := New(db, provider, secret)
		collector = c.(*Collector)
		collector.client = client
	})

	AfterEach(func() {
		if db != nil {
			db.Close(true)
		}
		if dbPath != "" {
			os.RemoveAll(filepath.Dir(dbPath))
		}
	})

	Describe("Name", func() {
		It("should return EC2", func() {
			Expect(collector.Name()).To(Equal("EC2"))
		})
	})

	Describe("HasParity", func() {
		It("should return false initially", func() {
			Expect(collector.HasParity()).To(BeFalse())
		})
	})

	Describe("collectInstances", func() {
		It("should collect instances and store in DB", func() {
			// Setup: Add instance to fake API
			instance := testutil.NewInstanceBuilder("i-1234567890abcdef0", "test-vm").
				WithInstanceType(ec2types.InstanceTypeT3Medium).
				WithState(ec2types.InstanceStateNameRunning).
				WithAvailabilityZone("us-east-1a").
				WithVolume("/dev/sda1", "vol-123").
				WithNetworkInterface("subnet-123", "vpc-123", "10.0.1.100").
				Build()
			fakeEC2.AddInstance(instance)

			// Execute
			err := collector.collectInstances(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: Check DB has the instance
			m := &model.Instance{Base: model.Base{UID: "i-1234567890abcdef0"}}
			err = db.Get(m)
			Expect(err).NotTo(HaveOccurred())
			Expect(m.Name).To(Equal("test-vm"))
			Expect(m.InstanceType).To(Equal("t3.medium"))
			Expect(m.State).To(Equal("running"))
			Expect(m.Platform).To(Equal("linux"))
			Expect(m.Kind).To(Equal("Instance"))
		})

		It("should handle multiple instances", func() {
			// Setup: Add multiple instances
			instance1 := testutil.NewInstanceBuilder("i-111", "vm-1").
				WithState(ec2types.InstanceStateNameRunning).
				Build()
			instance2 := testutil.NewInstanceBuilder("i-222", "vm-2").
				WithState(ec2types.InstanceStateNameStopped).
				Build()
			fakeEC2.AddInstance(instance1)
			fakeEC2.AddInstance(instance2)

			// Execute
			err := collector.collectInstances(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: Both instances should be in DB
			var instances []model.Instance
			err = db.List(&instances, libmodel.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(instances).To(HaveLen(2))
		})

		It("should update existing instances on change", func() {
			// Setup: Add instance and collect
			instance := testutil.NewInstanceBuilder("i-123", "test-vm").
				WithState(ec2types.InstanceStateNameRunning).
				Build()
			fakeEC2.AddInstance(instance)
			err := collector.collectInstances(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify initial state
			m := &model.Instance{Base: model.Base{UID: "i-123"}}
			err = db.Get(m)
			Expect(err).NotTo(HaveOccurred())
			Expect(m.State).To(Equal("running"))
			initialRevision := m.Revision
			Expect(initialRevision).To(BeNumerically(">=", 1))

			// Change instance state
			fakeEC2.SetInstanceState("i-123", ec2types.InstanceStateNameStopped)

			// Execute again
			err = collector.collectInstances(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: State should be updated, revision incremented
			m = &model.Instance{Base: model.Base{UID: "i-123"}}
			err = db.Get(m)
			Expect(err).NotTo(HaveOccurred())
			Expect(m.State).To(Equal("stopped"))
			Expect(m.Revision).To(BeNumerically(">", initialRevision))
		})

		It("should skip instances without ID", func() {
			// Setup: Add instance without ID (should be skipped)
			instance := ec2types.Instance{
				InstanceType: ec2types.InstanceTypeT2Micro,
				State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
			}
			fakeEC2.Instances[""] = instance

			// Execute - should not fail
			err := collector.collectInstances(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: No instances in DB
			var instances []model.Instance
			err = db.List(&instances, libmodel.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(instances).To(BeEmpty())
		})

		It("should handle API errors", func() {
			// Setup: Inject error
			fakeEC2.Errors[testutil.MethodDescribeInstances] = errAPIFailed

			// Execute
			err := collector.collectInstances(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should use instance ID as name when Name tag is missing", func() {
			// Setup: Add instance without Name tag
			instance := ec2types.Instance{
				InstanceId:   aws.String("i-noname"),
				InstanceType: ec2types.InstanceTypeT2Micro,
				State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
				Placement:    &ec2types.Placement{AvailabilityZone: aws.String("us-east-1a")},
				Tags:         []ec2types.Tag{}, // No Name tag
			}
			fakeEC2.AddInstance(instance)

			// Execute
			err := collector.collectInstances(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: Name should fallback to instance ID
			m := &model.Instance{Base: model.Base{UID: "i-noname"}}
			err = db.Get(m)
			Expect(err).NotTo(HaveOccurred())
			Expect(m.Name).To(Equal("i-noname"))
		})
	})

	Describe("collectVolumes", func() {
		It("should collect volumes and store in DB", func() {
			// Setup: Add volume to fake API
			volume := testutil.NewVolumeBuilder("vol-1234567890abcdef0").
				WithVolumeType(ec2types.VolumeTypeGp3).
				WithSize(100).
				WithState(ec2types.VolumeStateInUse).
				WithAvailabilityZone("us-east-1a").
				WithAttachment("i-123", "/dev/sda1").
				WithTag("Name", "test-volume").
				Build()
			fakeEC2.AddVolume(volume)

			// Execute
			err := collector.collectVolumes(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: Check DB has the volume
			m := &model.Volume{Base: model.Base{UID: "vol-1234567890abcdef0"}}
			err = db.Get(m)
			Expect(err).NotTo(HaveOccurred())
			Expect(m.Name).To(Equal("test-volume"))
			Expect(m.VolumeType).To(Equal("gp3"))
			Expect(m.State).To(Equal("in-use"))
			Expect(m.Size).To(Equal(int64(100)))
			Expect(m.Kind).To(Equal("Volume"))
		})

		It("should handle multiple volumes", func() {
			// Setup: Add multiple volumes
			volume1 := testutil.NewVolumeBuilder("vol-111").
				WithVolumeType(ec2types.VolumeTypeGp2).
				WithSize(50).
				Build()
			volume2 := testutil.NewVolumeBuilder("vol-222").
				WithVolumeType(ec2types.VolumeTypeIo1).
				WithSize(200).
				Build()
			fakeEC2.AddVolume(volume1)
			fakeEC2.AddVolume(volume2)

			// Execute
			err := collector.collectVolumes(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: Both volumes should be in DB
			var volumes []model.Volume
			err = db.List(&volumes, libmodel.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(2))
		})

		It("should update existing volumes on change", func() {
			// Setup: Add volume and collect
			volume := testutil.NewVolumeBuilder("vol-123").
				WithState(ec2types.VolumeStateAvailable).
				Build()
			fakeEC2.AddVolume(volume)
			err := collector.collectVolumes(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Get initial revision
			m := &model.Volume{Base: model.Base{UID: "vol-123"}}
			err = db.Get(m)
			Expect(err).NotTo(HaveOccurred())
			Expect(m.State).To(Equal("available"))
			initialRevision := m.Revision

			// Change volume state
			fakeEC2.SetVolumeState("vol-123", ec2types.VolumeStateInUse)

			// Execute again
			err = collector.collectVolumes(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: State should be updated
			m = &model.Volume{Base: model.Base{UID: "vol-123"}}
			err = db.Get(m)
			Expect(err).NotTo(HaveOccurred())
			Expect(m.State).To(Equal("in-use"))
			Expect(m.Revision).To(BeNumerically(">", initialRevision))
		})

		It("should handle API errors", func() {
			// Setup: Inject error
			fakeEC2.Errors[testutil.MethodDescribeVolumes] = errAPIFailed

			// Execute
			err := collector.collectVolumes(ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("collectNetworks", func() {
		It("should collect VPCs and store in DB", func() {
			// Setup: Add VPC to fake API
			vpc := testutil.NewVpcBuilder("vpc-123").
				WithCidrBlock("10.0.0.0/16").
				WithTag("Name", "test-vpc").
				Build()
			fakeEC2.AddVpc(vpc)

			// Execute
			err := collector.collectNetworks(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: Check DB has the VPC
			m := &model.Network{Base: model.Base{UID: "vpc-123"}}
			err = db.Get(m)
			Expect(err).NotTo(HaveOccurred())
			Expect(m.Name).To(Equal("test-vpc"))
			Expect(m.NetworkType).To(Equal("vpc"))
			Expect(m.CIDR).To(Equal("10.0.0.0/16"))
			Expect(m.Kind).To(Equal("Network"))
		})

		It("should collect subnets and store in DB", func() {
			// Setup: Add subnet to fake API
			subnet := testutil.NewSubnetBuilder("subnet-123", "vpc-123").
				WithCidrBlock("10.0.1.0/24").
				WithAvailabilityZone("us-east-1a").
				WithTag("Name", "test-subnet").
				Build()
			fakeEC2.AddSubnet(subnet)

			// Execute
			err := collector.collectNetworks(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: Check DB has the subnet
			m := &model.Network{Base: model.Base{UID: "subnet-123"}}
			err = db.Get(m)
			Expect(err).NotTo(HaveOccurred())
			Expect(m.Name).To(Equal("test-subnet"))
			Expect(m.NetworkType).To(Equal("subnet"))
			Expect(m.CIDR).To(Equal("10.0.1.0/24"))
		})

		It("should collect both VPCs and subnets", func() {
			// Setup: Add VPC and subnet
			vpc := testutil.NewVpcBuilder("vpc-123").Build()
			subnet := testutil.NewSubnetBuilder("subnet-123", "vpc-123").Build()
			fakeEC2.AddVpc(vpc)
			fakeEC2.AddSubnet(subnet)

			// Execute
			err := collector.collectNetworks(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: Both should be in DB
			var networks []model.Network
			err = db.List(&networks, libmodel.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(networks).To(HaveLen(2))
		})

		It("should handle VPC API errors", func() {
			// Setup: Inject error
			fakeEC2.Errors[testutil.MethodDescribeVpcs] = errAPIFailed

			// Execute
			err := collector.collectNetworks(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should handle Subnet API errors", func() {
			// Setup: Add VPC but inject subnet error
			vpc := testutil.NewVpcBuilder("vpc-123").Build()
			fakeEC2.AddVpc(vpc)
			fakeEC2.Errors[testutil.MethodDescribeSubnets] = errAPIFailed

			// Execute
			err := collector.collectNetworks(ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("collectStorageTypes", func() {
		It("should collect all EBS volume types", func() {
			// Execute
			err := collector.collectStorageTypes(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify: All storage types should be in DB
			var storages []model.Storage
			err = db.List(&storages, libmodel.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(storages).To(HaveLen(len(ebsVolumeTypes)))

			// Verify specific types exist
			expectedTypes := []string{"gp2", "gp3", "io1", "io2", "st1", "sc1", "standard"}
			for _, expectedType := range expectedTypes {
				found := false
				for _, s := range storages {
					if s.VolumeType == expectedType {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "Expected storage type %s not found", expectedType)
			}
		})

		It("should handle repeated collection of storage types", func() {
			// First collection
			err := collector.collectStorageTypes(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Get revision of gp3 after first collection
			m := &model.Storage{Base: model.Base{UID: "gp3"}}
			err = db.Get(m)
			Expect(err).NotTo(HaveOccurred())
			initialRevision := m.Revision
			Expect(initialRevision).To(BeNumerically(">=", 1))

			// Second collection
			err = collector.collectStorageTypes(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify storage type still exists with valid data
			m = &model.Storage{Base: model.Base{UID: "gp3"}}
			err = db.Get(m)
			Expect(err).NotTo(HaveOccurred())
			Expect(m.VolumeType).To(Equal("gp3"))
			Expect(m.Kind).To(Equal("Storage"))
		})
	})

	Describe("Collect", func() {
		It("should collect all resource types", func() {
			// Setup: Add resources
			instance := testutil.NewInstanceBuilder("i-123", "test-vm").Build()
			volume := testutil.NewVolumeBuilder("vol-123").Build()
			vpc := testutil.NewVpcBuilder("vpc-123").Build()
			subnet := testutil.NewSubnetBuilder("subnet-123", "vpc-123").Build()
			fakeEC2.AddInstance(instance)
			fakeEC2.AddVolume(volume)
			fakeEC2.AddVpc(vpc)
			fakeEC2.AddSubnet(subnet)

			// Execute
			err := collector.Collect()
			Expect(err).NotTo(HaveOccurred())

			// Verify: All resources should be in DB
			var instances []model.Instance
			err = db.List(&instances, libmodel.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(instances).To(HaveLen(1))

			var volumes []model.Volume
			err = db.List(&volumes, libmodel.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(1))

			var networks []model.Network
			err = db.List(&networks, libmodel.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(networks).To(HaveLen(2)) // VPC + subnet

			var storages []model.Storage
			err = db.List(&storages, libmodel.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(storages).To(HaveLen(len(ebsVolumeTypes)))
		})

		It("should succeed with partial failures", func() {
			// Setup: Inject instance error but volumes should succeed
			fakeEC2.Errors[testutil.MethodDescribeInstances] = errAPIFailed
			volume := testutil.NewVolumeBuilder("vol-123").Build()
			fakeEC2.AddVolume(volume)

			// Execute - should succeed (partial success)
			err := collector.Collect()
			Expect(err).NotTo(HaveOccurred())

			// Verify: Volumes should be collected despite instance failure
			var volumes []model.Volume
			err = db.List(&volumes, libmodel.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(1))
		})

		It("should succeed with partial failures when storageTypes succeeds", func() {
			// Setup: Inject errors for all AWS API calls
			// Note: collectStorageTypes doesn't call AWS APIs, so it always succeeds
			fakeEC2.Errors[testutil.MethodDescribeInstances] = errAPIFailed
			fakeEC2.Errors[testutil.MethodDescribeVolumes] = errAPIFailed
			fakeEC2.Errors[testutil.MethodDescribeVpcs] = errAPIFailed

			// Execute - should succeed because storageTypes (1/4) succeeds
			err := collector.Collect()
			Expect(err).NotTo(HaveOccurred())

			// Verify: Storage types should still be collected
			var storages []model.Storage
			err = db.List(&storages, libmodel.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(storages).To(HaveLen(len(ebsVolumeTypes)))
		})

		It("should prevent concurrent collections", func() {
			// Simulate collection in progress
			collector.mutex.Lock()
			collector.collecting = true
			collector.mutex.Unlock()

			// Execute - should return immediately
			err := collector.Collect()
			Expect(err).NotTo(HaveOccurred())

			// Reset
			collector.mutex.Lock()
			collector.collecting = false
			collector.mutex.Unlock()
		})
	})

	Describe("Helper functions", func() {
		table.DescribeTable("getNameFromTags",
			func(tags []ec2types.Tag, expected string) {
				Expect(getNameFromTags(tags)).To(Equal(expected))
			},
			table.Entry("returns name when Name tag exists",
				[]ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("my-vm")}},
				"my-vm"),
			table.Entry("returns empty when no Name tag",
				[]ec2types.Tag{{Key: aws.String("Environment"), Value: aws.String("prod")}},
				""),
			table.Entry("returns empty when tags are empty",
				[]ec2types.Tag{},
				""),
			table.Entry("returns empty when tags are nil",
				nil,
				""),
			table.Entry("handles multiple tags with Name",
				[]ec2types.Tag{
					{Key: aws.String("Environment"), Value: aws.String("prod")},
					{Key: aws.String("Name"), Value: aws.String("my-vm")},
					{Key: aws.String("Owner"), Value: aws.String("team-a")},
				},
				"my-vm"),
			table.Entry("handles nil key",
				[]ec2types.Tag{{Key: nil, Value: aws.String("value")}},
				""),
			table.Entry("handles nil value",
				[]ec2types.Tag{{Key: aws.String("Name"), Value: nil}},
				""),
		)

		table.DescribeTable("getPlatform",
			func(platform interface{}, platformDetails *string, expected string) {
				Expect(getPlatform(platform, platformDetails)).To(Equal(expected))
			},
			table.Entry("returns windows when platform is windows",
				"windows", nil, "windows"),
			table.Entry("returns windows when platformDetails contains Windows",
				nil, aws.String("Windows Server 2019"), "windows"),
			table.Entry("returns linux by default",
				nil, nil, "linux"),
			table.Entry("returns linux when platform is empty string",
				"", nil, "linux"),
			table.Entry("returns linux when platformDetails has no Windows",
				nil, aws.String("Linux/UNIX"), "linux"),
			table.Entry("prioritizes platform over platformDetails",
				"windows", aws.String("Linux/UNIX"), "windows"),
		)
	})
})

// Test context for collection methods
var ctx = context.Background()

// Common test error
var errAPIFailed = errors.New("API request failed")
