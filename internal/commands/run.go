package commands

import (
	"os"

	"github.com/barthv/kwir/internal/kwir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func newKwirCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:           "run",
		Short:         "Runs kwir admission webhook manager",
		SilenceErrors: true,
		SilenceUsage:  true,

		PreRun: func(cmd *cobra.Command, args []string) {
			// init controller-runtime logger for kwir manager
			log.SetLogger(zap.New())
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runKwirCommand(); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringP("config", "c", "./configs/kwir-config.yaml", "Path of the kwir yaml file holding rewrite rules")
	viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config"))

	cmd.PersistentFlags().StringP("tlsdir", "t", "./certs/", "Dir containing webhook's tls certificates")
	viper.BindPFlag("tlsdir", cmd.PersistentFlags().Lookup("tlsdir"))

	return &cmd
}

func runKwirCommand() error {
	logger := log.Log.WithName("kwir-manager")

	// Setup a Manager
	logger.Info("Setting up controller manager")
	mgrOpts := manager.Options{
		CertDir:                viper.GetString("tlsdir"),
		HealthProbeBindAddress: ":9080",
		LeaderElection:         false,
		LeaderElectionID:       "w1vraga9pn4pg3go.svc.kwir.cluster.local",
	}

	mgr, err := manager.New(config.GetConfigOrDie(), mgrOpts)
	if err != nil {
		logger.Error(err, "Unable to spawn controller manager")
		return err
	}

	// Setup webhooks
	logger.Info("Setting up webhook server")
	hookServer := mgr.GetWebhookServer()
	kwirPodRewriterHandler := &kwir.PodRewriter{Client: mgr.GetClient()}

	// Load Kwir configuration from file
	err = kwirPodRewriterHandler.LoadConfig(viper.GetString("config"))
	if err != nil {
		logger.Error(err, "Unable to load configuration")
		return err
	}

	logger.Info("Registering health & ready checks to the manager")
	err = mgr.AddReadyzCheck("readyz", healthz.Ping)
	if err != nil {
		logger.Error(err, "Unable add a readiness check")
		return err
	}
	err = mgr.AddHealthzCheck("healthz", healthz.Ping)
	if err != nil {
		logger.Error(err, "Unable add a health check")
		return err
	}

	logger.Info("Registering webhooks to the webhook server")
	hookServer.Register("/kwir-mutate-v1-pod", &webhook.Admission{Handler: kwirPodRewriterHandler})

	logger.Info("Starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		logger.Error(err, "Controller manager failed")
		os.Exit(1)
	}

	return nil
}
