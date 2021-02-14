package kwir

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// PodRewriter rewrites Pod images based on config rules
type PodRewriter struct {
	Client  client.Client
	decoder *admission.Decoder
	cfg     kwirConfig
}

// rewriteImages takes an input string and applies (in order) all rewrite rules then returns it
func (a *PodRewriter) rewriteImage(image string) (string, error) {
	return image, nil
}

// LoadConfig initialize kwir PodRewriter configuration from a given yaml file
func (a *PodRewriter) LoadConfig(cfgFile string) error {
	config, err := parseKwirConfig(cfgFile)
	if err != nil {
		return err
	}

	a.cfg = config
	return nil
}

// Handle is a kube webhook handler that rewrite Pod containers images based on its own config rules
func (a *PodRewriter) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.Log.WithName("kwir-podrewriter")
	pod := &corev1.Pod{}

	err := a.decoder.Decode(req, pod)

	// Fails and refuse admission if received object cannot be parsed as a core/v1/Pod
	if err != nil {
		logger.Error(err, "Webhook request must be a pod",
			"kind", req.Kind,
			"name", req.Name,
			"namespace", req.Namespace,
		)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// A special pod annotation allows to skip any kind of image mutation.
	if _, exists := pod.Annotations["kwir/podrewriter-skip"]; exists {
		logger.Info("Pod explitely skipped podrewriter handler",
			"namespace", pod.Namespace,
			"pod", pod.Name,
		)
		return admission.Allowed("Pod explitely skipped podrewriter policy")
	}

	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations["kwir/podrewriter-processed"] = "true"

	// rewrite any existing InitContainers
	for _, container := range pod.Spec.InitContainers {
		newImage, err := a.rewriteImage(container.Image)

		if err != nil {
			logger.Error(err, "Impossible to rewrite Container image",
				"namespace", pod.Namespace,
				"pod", pod.Name,
				"image", container.Image,
			)
			return admission.Errored(http.StatusInternalServerError, err)
		}

		logger.Info("Rewriting InitContainer image",
			"namespace", pod.Namespace,
			"pod", pod.Name,
			"original-image", container.Image,
			"mutated-image", newImage,
		)
	}

	// rewrite any existing Containers
	for _, container := range pod.Spec.Containers {
		newImage, err := a.rewriteImage(container.Image)

		if err != nil {
			logger.Error(err, "Impossible to rewrite Container image",
				"namespace", pod.Namespace,
				"pod", pod.Name,
				"image", container.Image,
			)
			return admission.Errored(http.StatusInternalServerError, err)
		}

		logger.Info("Rewriting Container image",
			"namespace", pod.Namespace,
			"pod", pod.Name,
			"original-image", container.Image,
			"mutated-image", newImage,
		)
	}

	// Prepare & apply pod mutation
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// PodRewriter implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (a *PodRewriter) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
