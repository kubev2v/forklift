package provider

import (
	"context"
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/dynamicserver"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/dynamic"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r Reconciler) EnsureDynamicProviderServer(ctx context.Context, provider *api.Provider) (err error) {
	ensurer := dynamicserver.Ensurer{Client: r.Client, Log: r.Log}

	// Find DynamicProviderServer by provider reference
	providerType := string(provider.Type())
	server, err := ensurer.FindProviderServerByProvider(ctx, provider.Name, provider.Namespace)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	// If no server exists, create one automatically
	if server == nil {
		r.Log.Info("No DynamicProviderServer found for provider, auto-creating",
			"provider", provider.Name,
			"namespace", provider.Namespace)

		err = r.CreateDynamicProviderServer(ctx, provider)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}

		// Server created, but it won't have status yet - will be updated on next reconcile
		return
	}

	// Set service reference with port from DynamicProviderServer spec
	if server.Status.Service != nil {
		servicePort := int32(8080) // Default port
		if server.Spec.Port != nil {
			servicePort = *server.Spec.Port
		}

		provider.Status.Service = &api.ServiceEndpoint{
			Name:      server.Status.Service.Name,
			Namespace: server.Status.Service.Namespace,
			Port:      &servicePort,
		}

		serviceURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
			server.Status.Service.Name,
			server.Status.Service.Namespace,
			servicePort)

		// Update service URL in registry
		// Don't fail the reconcile if this fails - will retry on next reconcile
		if updateErr := dynamic.Registry.UpdateServiceURL(providerType, serviceURL); updateErr != nil {
			r.Log.Info("Failed to update dynamic provider service URL (will retry)",
				"type", providerType,
				"error", updateErr)
		} else {
			r.Log.Info("Updated dynamic provider service URL",
				"type", providerType,
				"url", serviceURL)
		}
	}

	cnd := server.Status.FindCondition(dynamicserver.ApplianceManagementEnabled)
	if cnd != nil {
		provider.Status.SetCondition(*cnd)
	}

	// Copy features from DynamicProvider to Provider status
	err = r.copyFeaturesFromDynamicProvider(ctx, provider)
	if err != nil {
		r.Log.Info("Failed to copy features from DynamicProvider (will retry)",
			"error", err)
		// Don't fail the reconcile, will retry on next reconcile
		err = nil
	}

	return
}

// copyFeaturesFromDynamicProvider copies feature flags from DynamicProvider to Provider.Status
func (r Reconciler) copyFeaturesFromDynamicProvider(ctx context.Context, provider *api.Provider) error {
	providerType := string(provider.Type())

	// Find the DynamicProvider for this type
	dynamicProvider, err := r.findDynamicProviderByType(ctx, providerType)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Copy features if defined
	if dynamicProvider.Spec.Features != nil {
		provider.Status.Features = &api.ProviderFeatures{
			RequiresConversion:      dynamicProvider.Spec.Features.RequiresConversion,
			SupportedMigrationTypes: dynamicProvider.Spec.Features.SupportedMigrationTypes,
			SupportsCustomBuilder:   dynamicProvider.Spec.Features.SupportsCustomBuilder,
		}
		r.Log.Info("Copied features from DynamicProvider to Provider status",
			"provider", provider.Name,
			"type", providerType)
	} else {
		// No features defined, clear any existing features
		provider.Status.Features = nil
		r.Log.Info("No features defined in DynamicProvider",
			"provider", provider.Name,
			"type", providerType)
	}

	return nil
}

// findDynamicProviderByType finds a DynamicProvider by its type
func (r Reconciler) findDynamicProviderByType(ctx context.Context, providerType string) (*api.DynamicProvider, error) {
	dynamicProviderList := &api.DynamicProviderList{}
	err := r.List(ctx, dynamicProviderList, &client.ListOptions{})
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	for i := range dynamicProviderList.Items {
		dp := &dynamicProviderList.Items[i]
		if dp.Spec.Type == providerType {
			return dp, nil
		}
	}

	return nil, liberr.New(fmt.Sprintf("DynamicProvider not found for type: %s", providerType))
}

// CreateDynamicProviderServer creates a DynamicProviderServer CR for the given provider
func (r Reconciler) CreateDynamicProviderServer(ctx context.Context, provider *api.Provider) error {
	providerType := string(provider.Type())

	// Find the DynamicProvider for this type
	dynamicProvider, err := r.findDynamicProviderByType(ctx, providerType)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Get controller namespace from settings
	controllerNamespace := settings.Settings.Namespace
	if controllerNamespace == "" {
		controllerNamespace = "konveyor-forklift"
	}

	// Create DynamicProviderServer CR
	server := &api.DynamicProviderServer{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: providerType + "-",
			Namespace:    controllerNamespace,
			Labels: map[string]string{
				"app":          "forklift",
				"providerType": providerType,
				// Provider tracking labels for easy lookup and cleanup
				"forklift.konveyor.io/provider-name":      provider.Name,
				"forklift.konveyor.io/provider-namespace": provider.Namespace,
				"forklift.konveyor.io/provider-uid":       string(provider.UID),
				"forklift.konveyor.io/dynamic-provider":   dynamicProvider.Name,
			},
			Annotations: map[string]string{
				"forklift.konveyor.io/provider-ref": provider.Namespace + "/" + provider.Name,
			},
		},
		Spec: api.DynamicProviderServerSpec{
			DynamicProviderRef: api.ProviderReference{
				Name:      dynamicProvider.Name,
				Namespace: dynamicProvider.Namespace,
			},
			ProviderRef: api.ProviderReference{
				Name:      provider.Name,
				Namespace: provider.Namespace,
			},
		},
	}

	err = r.Create(ctx, server)
	if err != nil {
		if k8serr.IsAlreadyExists(err) {
			// Server already exists, this is fine
			r.Log.Info("DynamicProviderServer already exists",
				"provider", provider.Name)
			return nil
		}
		return liberr.Wrap(err)
	}

	r.Log.Info("Created DynamicProviderServer",
		"server", server.Name,
		"namespace", server.Namespace,
		"provider", provider.Name,
		"type", providerType)

	return nil
}

func (r Reconciler) DeleteDynamicProviderServer(ctx context.Context, provider *api.Provider) error {
	ensurer := dynamicserver.Ensurer{Client: r.Client, Log: r.Log}

	// Find ALL DynamicProviderServer CRs for this provider
	servers, err := ensurer.FindProviderServersByProvider(ctx, provider.Name, provider.Namespace)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Delete all found servers
	deletedCount := 0
	for i := range servers {
		server := &servers[i]
		deleteErr := r.Delete(ctx, server)
		if deleteErr != nil && !k8serr.IsNotFound(deleteErr) {
			r.Log.Error(deleteErr, "Failed to delete DynamicProviderServer",
				"server", server.Name,
				"namespace", server.Namespace,
				"provider", provider.Name)
			// Continue deleting other servers
			continue
		}
		r.Log.Info("Deleted DynamicProviderServer",
			"server", server.Name,
			"namespace", server.Namespace,
			"provider", provider.Name)
		deletedCount++
	}

	if deletedCount > 0 {
		r.Log.Info("Cleaned up DynamicProviderServer resources",
			"provider", provider.Name,
			"count", deletedCount)
	}

	return nil
}
