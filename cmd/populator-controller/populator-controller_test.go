package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("getVXPopulatorPodArgs", func() {
	It("should return the correct arguments", func() {
		xcopy := &v1beta1.VSphereXcopyVolumePopulator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-xcopy",
				Namespace: "test-namespace",
			},
			Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
				VmId:                 "test-vm-id",
				VmdkPath:             "test-vmdk-path",
				SecretName:           "test-secret-name",
				StorageVendorProduct: "test-storage-vendor-product",
				MigrationHost:        "test-migration-host",
			},
		}

		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(xcopy)
		Expect(err).NotTo(HaveOccurred())

		u := &unstructured.Unstructured{
			Object: unstructuredMap,
		}

		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pvc",
			},
		}

		args, err := getVXPopulatorPodArgs(false, u, pvc)
		Expect(err).NotTo(HaveOccurred())

		expectedArgs := []string{
			"--source-vm-id=test-vm-id",
			"--source-vmdk=test-vmdk-path",
			"--target-namespace=test-namespace",
			"--cr-name=test-xcopy",
			"--cr-namespace=test-namespace",
			"--owner-name=test-pvc",
			"--secret-name=test-secret-name",
			"--storage-vendor-product=test-storage-vendor-product",
			"--migration-host=test-migration-host",
		}

		Expect(args).To(Equal(expectedArgs))
	})

	It("should return the correct arguments when migration host is empty", func() {
		xcopy := &v1beta1.VSphereXcopyVolumePopulator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-xcopy",
				Namespace: "test-namespace",
			},
			Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
				VmId:                 "test-vm-id",
				VmdkPath:             "test-vmdk-path",
				SecretName:           "test-secret-name",
				StorageVendorProduct: "test-storage-vendor-product",
				MigrationHost:        "",
			},
		}

		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(xcopy)
		Expect(err).NotTo(HaveOccurred())

		u := &unstructured.Unstructured{
			Object: unstructuredMap,
		}

		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pvc",
			},
		}

		args, err := getVXPopulatorPodArgs(false, u, pvc)
		Expect(err).NotTo(HaveOccurred())

		expectedArgs := []string{
			"--source-vm-id=test-vm-id",
			"--source-vmdk=test-vmdk-path",
			"--target-namespace=test-namespace",
			"--cr-name=test-xcopy",
			"--cr-namespace=test-namespace",
			"--owner-name=test-pvc",
			"--secret-name=test-secret-name",
			"--storage-vendor-product=test-storage-vendor-product",
		}

		Expect(args).To(Equal(expectedArgs))
	})
})
