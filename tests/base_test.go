package main

import (
	"context"
	"fmt"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strconv"
	"testing"
	"time"

	"github.com/konveyor/forklift-controller/pkg/apis"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/provider"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/ovirt"
	ovirtsdk "github.com/ovirt/go-ovirt"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var namespace = "konveyor-forklift"

func TestSanityOvirtProvider(t *testing.T) {
	conf, err := config.GetConfig()
	if err != nil {
		fmt.Println("unable to set up client config")
		os.Exit(1)
	}

	logf.SetLogger(
		zap.New(zap.UseDevMode(false)))
	log := logf.Log.WithName("entrypoint")

	customEnv := false
	if customEnvVar := os.Getenv("OVIRT_CUSTOM_ENV"); customEnvVar == "true" {
		customEnv = true
	}
	username := os.Getenv("OVIRT_USERNAME")
	if username == "" {
		t.Fatal("OVIRT_USERNAME is not set")
	}
	password := os.Getenv("OVIRT_PASSWORD")
	if password == "" {
		t.Fatal("OVIRT_PASSWORD is not set")
	}

	ovirtURL := os.Getenv("OVIRT_URL")
	if ovirtURL == "" {
		t.Fatal("OVIRT_URL is not set")
	}

	cacertFile := os.Getenv("OVIRT_CACERT")
	if cacertFile == "" {
		t.Fatal("OVIRT_CACERT is not set")
	}

	fileinput, err := os.ReadFile(cacertFile)
	if err != nil {
		t.Fatalf("Could not read %s", cacertFile)
	}
	cacert := fmt.Sprintf("%s", fileinput)

	sc := os.Getenv("STORAGE_CLASS")
	if sc == "" {
		t.Fatal("STORAGE_CLASS is not set")
	}

	vmId := os.Getenv("OVIRT_VM_ID")
	if vmId == "" {
		t.Fatal("OVIRT_VM is not set")
	}
	migrationTimeout := "3"
	if timeoutInput := os.Getenv("MIGRATION_TIMEOUT"); timeoutInput != "" {
		migrationTimeout = timeoutInput
	}

	// Register
	err = v1beta1.SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Error(err, "Failed to build scheme")
	}
	err = apis.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Error(err, "Failed to add scheme")
	}

	cl, err := client.New(conf, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		t.Error(err, "Failed to create client")
	}

	log.Info("Creating secret...")

	secret := &corev1.Secret{
		TypeMeta: v1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Namespace: namespace,
			Name:      "ovirt-provider-test-secret",
			Labels: map[string]string{
				"createdForResource":     "ovirt-provider",
				"createdForResourceType": "providers",
				"createdForProviderType": "ovirt",
			},
		},
		Data: map[string][]byte{
			"user":     []byte(username),
			"password": []byte(password),
			"cacert":   []byte(cacert),
			"url":      []byte(ovirtURL),
		},
		Type: corev1.SecretTypeOpaque,
	}
	err = cl.Create(context.TODO(), secret, &client.CreateOptions{})
	if err != nil {
		t.Fatal(err, "Failed to create secret")
	}

	providerName := v1.ObjectMeta{
		Namespace: namespace,
		Name:      "ovirt-provider",
	}

	ovirtProvider := v1beta1.OVirt
	p := &v1beta1.Provider{
		TypeMeta: v1.TypeMeta{
			Kind:       "Provider",
			APIVersion: "forklift.konveyor.io/v1beta1",
		},
		ObjectMeta: providerName,
		Spec: v1beta1.ProviderSpec{
			Type: &ovirtProvider,
			URL:  ovirtURL,
			Secret: corev1.ObjectReference{
				Name:      secret.Name,
				Namespace: namespace,
			},
		},
	}

	err = cl.Create(context.TODO(), p, &client.CreateOptions{})
	if err != nil {
		t.Fatal(err, "Failed to create ovirt provider")
	}

	returnedProvider := &v1beta1.Provider{}
	providerIdentifier := types.NamespacedName{Namespace: providerName.Namespace, Name: providerName.Name}
	err = cl.Get(context.TODO(), providerIdentifier, returnedProvider)
	if err != nil {
		t.Fatal(err, "Failed to get ovirt provider")
	}

	done := make(chan struct{})

	// Wait for provider to be ready
	statusCheck := func() {
		err = cl.Get(context.TODO(), providerIdentifier, returnedProvider)
		if err != nil {
			t.Fatal(err, "Failed to create ovirt provider")
		}

		if returnedProvider.Status.Conditions.IsReady() {
			log.Info("oVirt Provider is ready")
			close(done)
		}
	}

	go wait.Until(statusCheck, time.Second, done)

	timeout := time.After(1 * time.Minute)
	select {
	case <-timeout:
		t.Errorf("Provider is not ready in time, last status: %v", returnedProvider.Status)
	case <-done:
	}

	// sdPairs set with the default settings for kind CI.
	sdPairs := []v1beta1.StoragePair{
		{
			Source: ref.Ref{ID: "95ef6fee-5773-46a2-9340-a636958a96b8"},
			Destination: v1beta1.DestinationStorage{
				StorageClass: sc,
			},
		},
	}
	// nicPairs set with the default settings for kind CI.
	nicPairs := []v1beta1.NetworkPair{
		{
			Source: ref.Ref{ID: "6b6b7239-5ea1-4f08-a76e-be150ab8eb89"},
			Destination: v1beta1.DestinationNetwork{
				Type: "pod",
			},
		},
	}

	if customEnv {
		ovirtCl := &Client{}
		defer ovirtCl.Close()

		err = ovirtCl.connect(secret, ovirtURL)
		if err != nil {
			t.Fatal(err, "Failed to connect ovirt sdk")
		}
		_, vmService, err := ovirtCl.getVMs(ref.Ref{ID: vmId})
		if err != nil {
			t.Fatal(err, "Failed to get vm")
		}

		diskAttachementResponse, err := vmService.DiskAttachmentsService().List().Send()
		if err != nil {
			t.Fatal(err, "Failed to get disk attachment service")
		}

		disks, ok := diskAttachementResponse.Attachments()
		sdPairs := []v1beta1.StoragePair{}
		if !ok {
			t.Fatal("Failed to get disks")
		}
		for _, da := range disks.Slice() {
			disk, ok := da.Disk()
			if !ok {
				t.Fatal("Failed to get disk")
			}
			diskService := ovirtCl.connection.SystemService().DisksService().DiskService(disk.MustId())
			diskResponse, err := diskService.Get().Send()
			if err != nil {
				t.Fatal(err, "Failed to get disk")
			}
			sds, ok := diskResponse.MustDisk().StorageDomains()
			if !ok {
				t.Fatal("Failed to get storage domains")
			}
			for _, sd := range sds.Slice() {
				sdId, ok := sd.Id()
				if !ok {
					t.Fatal("Failed to get storage domain id")
				}
				pair := v1beta1.StoragePair{
					Source: ref.Ref{ID: sdId},
					Destination: v1beta1.DestinationStorage{
						StorageClass: sc,
					},
				}
				sdPairs = append(sdPairs, pair)
			}
		}
		nicsResponse, err := vmService.NicsService().List().Send()
		nics, ok := nicsResponse.Nics()
		if !ok {
			t.Fatal("Failed to get nics")
		}
		nicPairs := []v1beta1.NetworkPair{}
		for _, nic := range nics.Slice() {
			vnicService := ovirtCl.connection.SystemService().VnicProfilesService().ProfileService(nic.MustVnicProfile().MustId())
			vnicResponse, err := vnicService.Get().Send()
			if err != nil {
				t.Fatal(err, "Failed to get vnic service")
			}
			profile, ok := vnicResponse.Profile()
			if !ok {
				t.Fatal("Failed to get nic profile")
			}

			network, ok := profile.Network()
			if !ok {
				t.Fatal("Failed to get network")
			}
			networkId, ok := network.Id()
			if !ok {
				t.Fatal("Failed to get network id")
			}
			pair := v1beta1.NetworkPair{
				Source: ref.Ref{ID: networkId},
				Destination: v1beta1.DestinationNetwork{
					Type: "pod",
				},
			}
			nicPairs = append(nicPairs, pair)
		}
	}

	storageMap := &v1beta1.StorageMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "StorageMap",
			APIVersion: "forklift.konveyor.io/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "storage-map-test",
			Namespace: namespace,
		},
		Spec: v1beta1.StorageMapSpec{
			Map: sdPairs,
			Provider: provider.Pair{
				Destination: corev1.ObjectReference{
					Name:      "host",
					Namespace: namespace,
				},
				Source: corev1.ObjectReference{
					Name:      providerIdentifier.Name,
					Namespace: providerIdentifier.Namespace,
				}},
		},
	}

	log.Info("Creating Storage Map...")
	err = cl.Create(context.TODO(), storageMap, &client.CreateOptions{})
	if err != nil {
		t.Fatal(err, "Failed to create storagemap")
	}

	networkMap := &v1beta1.NetworkMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "NetworkMap",
			APIVersion: "forklift.konveyor.io/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "network-map-test",
			Namespace: namespace,
		},
		Spec: v1beta1.NetworkMapSpec{
			Map: nicPairs,
			Provider: provider.Pair{
				Destination: corev1.ObjectReference{
					Name:      "host",
					Namespace: namespace,
				},
				Source: corev1.ObjectReference{
					Name:      providerIdentifier.Name,
					Namespace: providerIdentifier.Namespace,
				}},
		},
	}

	log.Info("Creating Network Map...")
	err = cl.Create(context.TODO(), networkMap, &client.CreateOptions{})
	if err != nil {
		t.Fatal(err, "Failed to create networkmap")
	}

	plan := &v1beta1.Plan{
		TypeMeta: v1.TypeMeta{
			Kind:       "Plan",
			APIVersion: "forklift.konveyor.io/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "plan-test",
			Namespace: namespace,
		},
		Spec: v1beta1.PlanSpec{
			Provider: provider.Pair{
				Destination: corev1.ObjectReference{
					Name:      "host",
					Namespace: namespace,
				},
				Source: corev1.ObjectReference{
					Name:      providerIdentifier.Name,
					Namespace: providerIdentifier.Namespace,
				},
			},
			Archived:        false,
			Warm:            false,
			TargetNamespace: providerIdentifier.Namespace,
			Map: plan.Map{
				Storage: corev1.ObjectReference{
					Name:      "storage-map-test",
					Namespace: namespace,
				},
				Network: corev1.ObjectReference{
					Name:      "network-map-test",
					Namespace: namespace,
				},
			},
			VMs: []plan.VM{
				{
					Ref: ref.Ref{
						//ID: vm.MustId(),
						ID: vmId,
					},
				},
			},
		},
	}

	log.Info("Creating Plan...")
	err = cl.Create(context.TODO(), plan, &client.CreateOptions{})
	if err != nil {
		t.Fatal(err, "Failed to create plan")
	}

	done = make(chan struct{})

	returnedPlan := &v1beta1.Plan{}
	planIdentifier := types.NamespacedName{Namespace: plan.Namespace, Name: plan.Name}
	// Wait for plan to be ready
	statusCheck = func() {
		err = cl.Get(context.TODO(), planIdentifier, returnedPlan)
		if err != nil {
			t.Fatal(err, "Failed to get plan")
		}

		if returnedPlan.Status.Conditions.IsReady() {
			log.Info("Plan is ready")
			close(done)
		}
	}

	go wait.Until(statusCheck, time.Second, done)

	select {
	case <-timeout:
		t.Errorf("Plan is not ready in time, last status: %v", returnedPlan.Status)
	case <-done:
	}

	migration := &v1beta1.Migration{
		TypeMeta: v1.TypeMeta{
			Kind:       "Migration",
			APIVersion: "forklift.konveyor.io/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "migration-test",
			Namespace: namespace,
		},
		Spec: v1beta1.MigrationSpec{
			Plan: corev1.ObjectReference{
				Name:      "plan-test",
				Namespace: namespace,
			},
		},
	}

	log.Info("Creating the migration...")
	done = make(chan struct{})

	err = cl.Create(context.TODO(), migration, &client.CreateOptions{})
	if err != nil {
		t.Fatal(err, "Failed to create migration")
	}

	returnedMigration := &v1beta1.Migration{}
	migrationIdentifier := types.NamespacedName{Namespace: migration.Namespace, Name: migration.Name}
	// Wait for migration to be ready
	statusCheck = func() {
		err = cl.Get(context.TODO(), migrationIdentifier, returnedMigration)
		if err != nil {
			t.Fatal(err, "Failed to get migration")
		}

		if returnedMigration.Status.Conditions.IsReady() {
			log.Info("Migration is ready")
			close(done)
		}
	}

	go wait.Until(statusCheck, time.Second, done)

	select {
	case <-timeout:
		t.Errorf("Migration is not ready in time, last status: %v", returnedMigration.Status)
	case <-done:
	}

	// Wait for migration to end
	done = make(chan struct{})
	statusCheck = func() {
		err = cl.Get(context.TODO(), migrationIdentifier, returnedMigration)
		if err != nil {
			t.Fatal(err, "Failed to get migration")
		}

		if condition := returnedMigration.Status.Conditions.FindCondition("Succeeded"); condition != nil {
			log.Info("Migration succeeded")
			close(done)
		}
		if condition := returnedMigration.Status.Conditions.FindCondition("Failed"); condition != nil {
			t.Error("Migration failed")
			close(done)
			t.Fatalf("migration failed %v", returnedMigration.Status.VMs[0].Error.Reasons)
		}
	}

	go wait.Until(statusCheck, time.Second, done)
	migrationTimeoutInt, _ := strconv.Atoi(migrationTimeout)
	timeout = time.After(time.Duration(migrationTimeoutInt) * time.Minute)

	select {
	case <-timeout:
		t.Errorf("Migration is not ready done time, last status: %v", returnedMigration.Status)
	case <-done:
	}
}

// Connect to the oVirt API.
func (r *Client) connect(secret *corev1.Secret, url string) (err error) {
	r.connection, err = ovirtsdk.NewConnectionBuilder().
		URL(url).
		Username(r.user(*secret)).
		Password(r.password(*secret)).
		CACert(r.cacert(*secret)).
		Insecure(ovirt.GetInsecureSkipVerifyFlag(secret)).
		Build()
	if err != nil {
		return err
	}
	return
}

func (r *Client) user(secret corev1.Secret) string {
	if user, found := secret.Data["user"]; found {
		return string(user)
	}
	return ""
}

func (r *Client) password(secret corev1.Secret) string {
	if password, found := secret.Data["password"]; found {
		return string(password)
	}
	return ""
}

func (r *Client) cacert(secret corev1.Secret) []byte {
	if cacert, found := secret.Data["cacert"]; found {
		return cacert
	}
	return nil
}

// Get the VM by ref.
func (r *Client) getVMs(vmRef ref.Ref) (ovirtVm *ovirtsdk.Vm, vmService *ovirtsdk.VmService, err error) {
	vmService = r.connection.SystemService().VmsService().VmService(vmRef.ID)
	vmResponse, err := vmService.Get().Send()
	if err != nil {
		return
	}
	ovirtVm, ok := vmResponse.Vm()
	if !ok {
		err = fmt.Errorf(
			"VM %s source lookup failed",
			vmRef.String())
	}
	return
}

// Close the connection to the oVirt API.
func (r *Client) Close() {
	if r.connection != nil {
		_ = r.connection.Close()
		r.connection = nil
	}
}

// oVirt VM Client
type Client struct {
	connection *ovirtsdk.Connection
}
