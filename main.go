/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	// +kubebuilder:scaffold:imports

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	cgrecord "k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/klog/klogr"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha3"
	infrav1controllersexp "sigs.k8s.io/cluster-api-provider-gcp/controllers/exp"
	"sigs.k8s.io/cluster-api-provider-gcp/controllers"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	klog.InitFlags(nil)

	_ = clientgoscheme.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

var (
	enableLeaderElection    bool
	metricsAddr             string
	leaderElectionNamespace string
	watchNamespace          string
	profilerAddress         string
	healthAddr              string
	gcpClusterConcurrency   int
	gcpMachineConcurrency   int
	webhookPort             int
	reconcileTimeout        time.Duration
	syncPeriod              time.Duration
)

func main() {
	initFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	if watchNamespace != "" {
		setupLog.Info("Watching cluster-api objects only in namespace for reconciliation", "namespace", watchNamespace)
	}

	if profilerAddress != "" {
		setupLog.Info("Profiler listening for requests", "profiler-address", profilerAddress)
		go func() {
			setupLog.Error(http.ListenAndServe(profilerAddress, nil), "listen and serve error")
		}()
	}

	ctrl.SetLogger(klogr.New())

	// Machine and cluster operations can create enough events to trigger the event recorder spam filter
	// Setting the burst size higher ensures all events will be recorded and submitted to the API
	broadcaster := cgrecord.NewBroadcasterWithCorrelatorOptions(cgrecord.CorrelatorOptions{
		BurstSize: 100,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "controller-leader-election-capg",
		LeaderElectionNamespace: leaderElectionNamespace,
		SyncPeriod:              &syncPeriod,
		Namespace:               watchNamespace,
		Port:                    webhookPort,
		HealthProbeBindAddress:  healthAddr,
		EventBroadcaster:        broadcaster,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize event recorder.
	record.InitFromRecorder(mgr.GetEventRecorderFor("gcp-controller"))

	if webhookPort == 0 {
		if (false) {
			if err = (&controllers.GCPMachineReconciler{
				Client:           mgr.GetClient(),
				Log:              ctrl.Log.WithName("controllers").WithName("GCPMachine"),
				ReconcileTimeout: reconcileTimeout,
			}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: gcpMachineConcurrency}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "GCPMachine")
				os.Exit(1)
			}
			if err = (&controllers.GCPClusterReconciler{
				Client:           mgr.GetClient(),
				Log:              ctrl.Log.WithName("controllers").WithName("GCPCluster"),
				ReconcileTimeout: reconcileTimeout,
			}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: gcpClusterConcurrency}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "GCPCluster")
				os.Exit(1)
			}
		} else {
			if err = (&infrav1controllersexp.GKEMachinePoolReconciler{
				Client: mgr.GetClient(),
				Log:              ctrl.Log.WithName("controllers").WithName("GKEMachine"),
				ReconcileTimeout: reconcileTimeout,
			}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: gcpMachineConcurrency}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "GKEMachine")
				os.Exit(1)
			}
			if err = (&infrav1controllersexp.GKEClusterReconciler{
				Client:           mgr.GetClient(),
				Log:              ctrl.Log.WithName("controllers").WithName("GKECluster"),
				ReconcileTimeout: reconcileTimeout,
			}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: gcpClusterConcurrency}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "GKECluster")
				os.Exit(1)
			}
		}
	} else {
		if err = (&infrav1.GCPMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "GCPMachineTemplate")
			os.Exit(1)
		}
		if err = (&infrav1.GCPMachine{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "GCPMachine")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func initFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&metricsAddr,
		"metrics-addr",
		":8080",
		"The address the metric endpoint binds to.",
	)

	fs.BoolVar(
		&enableLeaderElection,
		"enable-leader-election",
		false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.",
	)

	fs.StringVar(
		&watchNamespace,
		"namespace",
		"",
		"Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.",
	)

	fs.StringVar(
		&leaderElectionNamespace,
		"leader-election-namespace",
		"",
		"Namespace that the controller performs leader election in. If unspecified, the controller will discover which namespace it is running in.",
	)

	fs.StringVar(
		&profilerAddress,
		"profiler-address",
		"",
		"Bind address to expose the pprof profiler (e.g. localhost:6060)",
	)

	fs.IntVar(&gcpClusterConcurrency,
		"gcpcluster-concurrency",
		10,
		"Number of GCPClusters to process simultaneously",
	)

	fs.IntVar(&gcpMachineConcurrency,
		"gcpmachine-concurrency",
		10,
		"Number of GCPMachines to process simultaneously",
	)

	fs.DurationVar(&syncPeriod,
		"sync-period",
		10*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)",
	)

	fs.IntVar(&webhookPort,
		"webhook-port",
		0,
		"Webhook Server port, disabled by default. When enabled, the manager will only work as webhook server, no reconcilers are installed.",
	)

	fs.StringVar(&healthAddr,
		"health-addr",
		":9440",
		"The address the health endpoint binds to.",
	)

	fs.DurationVar(&reconcileTimeout,
		"reconcile-timeout",
		reconciler.DefaultLoopTimeout,
		"The maximum duration a reconcile loop can run (e.g. 90m)",
	)
}
