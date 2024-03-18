/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	schemaregmock "kafka-schema-operator/schemareg-mock"
	"net/url"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kafkaschemaoperatorv2beta1 "kafka-schema-operator/api/v2beta1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg          *rest.Config
	k8sClient    client.Client
	testEnv      *envtest.Environment
	srMockServer *ghttp.Server
	srMock       *schemaregmock.SchemaRegMock
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	Expect(setupKubeBuilderAssets()).To(Succeed())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = kafkaschemaoperatorv2beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	By("Starting Schema Registry Mock")
	srMock = schemaregmock.NewSchemaRegMock(
		map[string]schemaregmock.SchemaCompatibilityPolicy{},
		ctrl.Log,
	)
	srMockServer = srMock.GetServer()
	srMockServerUrl, err := url.ParseRequestURI(srMockServer.HTTPTestServer.URL)
	Expect(err).ToNot(HaveOccurred())
	Expect(os.Setenv("SCHEMA_REGISTRY_HOST", srMockServerUrl.Hostname())).To(Succeed())
	Expect(os.Setenv("SCHEMA_REGISTRY_PORT", srMockServerUrl.Port())).To(Succeed())

	By("Run Controllers")
	options := ctrl.Options{
		Scheme:    scheme.Scheme,
		Port:      9443,
		Namespace: "default",
	}

	k8sManager, err := ctrl.NewManager(cfg, options)
	Expect(err).ToNot(HaveOccurred())

	err = (&KafkaSchemaReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
		gexec.KillAndWait(4 * time.Second)

		// Teardown the test environment once controller is finished.
		// Otherwise, from Kubernetes 1.21+, teardown timeouts waiting on
		// kube-apiserver to return
		By("tearing down the test environment")
		err := testEnv.Stop()
		Expect(err).ToNot(HaveOccurred())
		srMockServer.Close()
	}()
})

func setupKubeBuilderAssets() error {
	kubeBuilderAssets, err := os.ReadDir("../bin/k8s")
	Expect(err).NotTo(HaveOccurred())
	var kubeBuilderAssetsPath string
	for _, dirEntry := range kubeBuilderAssets {
		if dirEntry.IsDir() {
			kubeBuilderAssetsPath = "../bin/k8s/" + dirEntry.Name()
			break
		}
	}
	if len(kubeBuilderAssetsPath) == 0 {
		return errors.New("kubeBuilderAssetsPath not found")
	}

	ctrl.Log.Info("Setting env KUBEBUILDER_ASSETS=" + kubeBuilderAssetsPath)
	return os.Setenv("KUBEBUILDER_ASSETS", kubeBuilderAssetsPath)
}
