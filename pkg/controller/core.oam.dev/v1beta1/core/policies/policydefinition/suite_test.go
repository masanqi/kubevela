/*
 Copyright 2021 The KubeVela Authors.

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

package policydefinition

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	oamCore "github.com/oam-dev/kubevela/apis/core.oam.dev"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var controllerDone context.CancelFunc
var r Reconciler
var defRevisionLimit = 5

func TestPolicyDefinition(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PolicyDefinition Suite")
}

var _ = BeforeSuite(func() {
	By("Bootstrapping test environment")
	useExistCluster := false
	testEnv = &envtest.Environment{
		ControlPlaneStartTimeout: time.Minute,
		ControlPlaneStopTimeout:  time.Minute,
		CRDDirectoryPaths: []string{
			filepath.Join("../../../../../../..", "charts/vela-core/crds"), // this has all the required CRDs,
		},
		UseExistingCluster: &useExistCluster,
	}
	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	Expect(oamCore.AddToScheme(scheme.Scheme)).Should(BeNil())
	Expect(crdv1.AddToScheme(scheme.Scheme)).Should(BeNil())

	By("Create the k8s client")
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	By("Starting the controller in the background")
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		WebhookServer: ctrlwebhook.NewServer(ctrlwebhook.Options{
			Port: 48081,
		}),
	})
	Expect(err).ToNot(HaveOccurred())

	r = Reconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		defRevLimit: defRevisionLimit,
	}
	Expect(r.SetupWithManager(mgr)).ToNot(HaveOccurred())
	var ctx context.Context
	ctx, controllerDone = context.WithCancel(context.Background())
	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(ctx)).ToNot(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	By("Stop the controller")
	controllerDone()

	By("Tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func ReconcileRetry(r reconcile.Reconciler, req reconcile.Request) {
	Eventually(func() error {
		_, err := r.Reconcile(context.TODO(), req)
		return err
	}, 15*time.Second, time.Second).Should(BeNil())
}
