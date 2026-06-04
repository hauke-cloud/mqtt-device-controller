package main

import (
	"flag"
	"log/slog"
	"os"

	iov1 "github.com/hauke-cloud/mqtt-device-controller/api/v1alpha1"
	"github.com/hauke-cloud/mqtt-device-controller/internal/api"
	k8sctrl "github.com/hauke-cloud/mqtt-device-controller/internal/k8s"
	mqttmgr "github.com/hauke-cloud/mqtt-device-controller/internal/mqtt"
	"github.com/hauke-cloud/mqtt-device-controller/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var scheme = k8sruntime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(iov1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr   string
		probeAddr     string
		apiAddr       string
		apiCertFile   string
		apiKeyFile    string
		apiCAFile     string
		namespace     string
		enableLeader  bool
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Address for the controller-runtime metrics endpoint.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Address for health probes.")
	flag.StringVar(&apiAddr, "api-bind-address", ":8443", "Address for the REST API (mTLS).")
	flag.StringVar(&apiCertFile, "tls-cert-file", "/tls/tls.crt", "Path to the server TLS certificate.")
	flag.StringVar(&apiKeyFile, "tls-key-file", "/tls/tls.key", "Path to the server TLS key.")
	flag.StringVar(&apiCAFile, "tls-ca-file", "/tls/ca.crt", "Path to the CA cert for client cert verification.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch for devices. Defaults to all namespaces.")
	flag.BoolVar(&enableLeader, "leader-elect", false, "Enable leader election for controller manager.")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))

	m := metrics.New(ctrlmetrics.Registry)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeader,
		LeaderElectionID:       "mqtt-device-controller-leader",
	})
	if err != nil {
		log.Error("unable to start manager", "err", err)
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error("unable to set up health check", "err", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error("unable to set up ready check", "err", err)
		os.Exit(1)
	}

	// Index MQTTDevice by spec.bridgeRef.name so markMissingDevicesStale can list efficiently.
	if err := mgr.GetFieldIndexer().IndexField(
		ctrl.SetupSignalHandler(),
		&iov1.MQTTDevice{},
		"spec.bridgeRef.name",
		func(obj client.Object) []string {
			dev := obj.(*iov1.MQTTDevice)
			return []string{dev.Spec.BridgeRef.Name}
		},
	); err != nil {
		log.Error("unable to set up field index", "err", err)
		os.Exit(1)
	}

	mqttManager := mqttmgr.NewManager(mgr.GetClient(), log, m)

	if err := k8sctrl.NewBridgeReconciler(mgr.GetClient(), log, mqttManager).
		SetupWithManager(mgr); err != nil {
		log.Error("unable to create BridgeReconciler", "err", err)
		os.Exit(1)
	}

	if err := k8sctrl.NewDeviceReconciler(mgr.GetClient(), log, mqttManager, m).
		SetupWithManager(mgr); err != nil {
		log.Error("unable to create DeviceReconciler", "err", err)
		os.Exit(1)
	}

	apiServer, err := api.NewServer(api.ServerConfig{
		Addr:     apiAddr,
		CertFile: apiCertFile,
		KeyFile:  apiKeyFile,
		CAFile:   apiCAFile,
	}, api.NewHandler(mgr.GetClient(), namespace), log)
	if err != nil {
		log.Error("unable to create API server", "err", err)
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	go func() {
		if err := apiServer.Start(ctx); err != nil {
			log.Error("API server error", "err", err)
			os.Exit(1)
		}
	}()

	log.Info("starting controller manager")
	if err := mgr.Start(ctx); err != nil {
		log.Error("manager exited with error", "err", err)
		os.Exit(1)
	}
}

// ensure prometheus.Registry satisfies the interface used by metrics.New
var _ prometheus.Registerer = prometheus.DefaultRegisterer
