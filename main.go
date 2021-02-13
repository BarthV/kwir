package main

import (
	"os"

	"github.com/barthv/kwir/internal/kwir"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func init() {
	log.SetLogger(zap.New())
}

func main() {
	logger := log.Log.WithName("kwir-manager")

	// Setup a Manager
	logger.Info("setting up manager")
	mgrOpts := manager.Options{
		CertDir: "/certs",
	}

	mgr, err := manager.New(config.GetConfigOrDie(), mgrOpts)
	if err != nil {
		logger.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	// Setup webhooks
	logger.Info("setting up webhook server")
	hookServer := mgr.GetWebhookServer()

	kwirPodRewriterHandler := &kwir.PodRewriter{
		Client: mgr.GetClient(),
	}

	logger.Info("registering webhooks to the webhook server")
	hookServer.Register("/kwir-mutate-v1-pod", &webhook.Admission{Handler: kwirPodRewriterHandler})

	logger.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		logger.Error(err, "unable to run manager")
		os.Exit(1)
	}
}
