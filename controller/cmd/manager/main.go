/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

// Command manager runs the KubeTraffic controller.
package main

import (
	"flag"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
	"github.com/kubetraffic/controller/internal/controller"
	"github.com/kubetraffic/controller/internal/controlplane"
	"github.com/kubetraffic/controller/internal/proxy"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(trafficv1alpha1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr           string
		probeAddr             string
		enableLeaderElection  bool
		leaderElectionID      string
		resyncInterval        time.Duration
		cacheSyncPeriod       time.Duration
		dataplaneURL          string
		dataplaneUsername     string
		dataplanePasswordFile string
		controlPlaneURL       string
		controlPlaneTimeout   time.Duration
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Address the metrics endpoint binds to; '0' disables it.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Address the health/readiness probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election, ensuring only one active manager.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "kubetraffic-controller", "Name of the leader-election Lease.")
	flag.DurationVar(&resyncInterval, "resync-interval", 10*time.Minute, "Per-TrafficRoute periodic reconcile interval.")
	flag.DurationVar(&cacheSyncPeriod, "cache-sync-period", 30*time.Minute, "Informer full-resync period.")
	flag.StringVar(&dataplaneURL, "haproxy-dataplane-url", "", "HAProxy Data Plane API root (e.g. http://host:5555). Empty disables data-plane programming.")
	flag.StringVar(&dataplaneUsername, "haproxy-dataplane-username", "admin", "HAProxy Data Plane API username.")
	flag.StringVar(&dataplanePasswordFile, "haproxy-dataplane-password-file", "", "Path to a file containing the Data Plane API password.")
	flag.StringVar(&controlPlaneURL, "control-plane-grpc-url", "", "Java control-plane gRPC address (e.g. host:9090). Empty disables control-plane registration.")
	flag.DurationVar(&controlPlaneTimeout, "control-plane-grpc-timeout", 5*time.Second, "Per-RPC timeout for control-plane calls.")

	zapOpts := zap.Options{Development: false}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	var dataPlane proxy.Proxy
	if dataplaneURL != "" {
		password, err := os.ReadFile(dataplanePasswordFile)
		if err != nil {
			setupLog.Error(err, "unable to read --haproxy-dataplane-password-file")
			os.Exit(1)
		}
		dataPlane = proxy.NewHAProxy(proxy.HAProxyOptions{
			BaseURL:  dataplaneURL,
			Username: dataplaneUsername,
			Password: strings.TrimSpace(string(password)),
		})
		setupLog.Info("data-plane programming enabled", "dataplaneURL", dataplaneURL)
	} else {
		setupLog.Info("data-plane programming disabled (no --haproxy-dataplane-url)")
	}

	var cpClient *controlplane.GRPCClient
	var decisionEvents chan event.GenericEvent
	if controlPlaneURL != "" {
		var dialErr error
		cpClient, dialErr = controlplane.Dial(controlPlaneURL, controlPlaneTimeout)
		if dialErr != nil {
			setupLog.Error(dialErr, "unable to dial control plane")
			os.Exit(1)
		}
		decisionEvents = make(chan event.GenericEvent, 64)
		setupLog.Info("control-plane registration enabled", "controlPlaneURL", controlPlaneURL)
	} else {
		setupLog.Info("control-plane registration disabled (no --control-plane-grpc-url)")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                        scheme,
		Metrics:                       metricsserver.Options{BindAddress: metricsAddr, SecureServing: false},
		HealthProbeBindAddress:        probeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              leaderElectionID,
		LeaderElectionReleaseOnCancel: true,
		Cache:                         cache.Options{SyncPeriod: &cacheSyncPeriod},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	reconciler := &controller.TrafficRouteReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Recorder:       mgr.GetEventRecorderFor("trafficroute-controller"),
		ResyncInterval: resyncInterval,
		Proxy:          dataPlane,
		DecisionEvents: decisionEvents,
	}
	if cpClient != nil {
		reconciler.ControlPlane = cpClient
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TrafficRoute")
		os.Exit(1)
	}

	if cpClient != nil {
		watcher := &controller.DecisionWatcher{Client: cpClient, Trigger: decisionEvents}
		if err := mgr.Add(watcher); err != nil {
			setupLog.Error(err, "unable to add decision watcher")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager", "leaderElection", enableLeaderElection, "resyncInterval", resyncInterval.String())
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
