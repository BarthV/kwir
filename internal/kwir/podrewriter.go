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
}

func (a *PodRewriter) rewriteImage(image string) (string, error) {
	return image, nil
}

// Handle is a kube webhook handler that rewrite Pod containers images based on its own config rules
func (a *PodRewriter) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.Log.WithName("kwir-podrewriter")
	pod := &corev1.Pod{}

	err := a.decoder.Decode(req, pod)
	if err != nil {
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

	for _, container := range pod.Spec.InitContainers {
		newImage, _ := a.rewriteImage(container.Image)
		logger.Info("Rewriting InitContainer image",
			"namespace", pod.Namespace,
			"pod", pod.Name,
			"original-image", container.Image,
			"mutated-image", newImage,
		)
	}

	for _, container := range pod.Spec.Containers {
		newImage, _ := a.rewriteImage(container.Image)
		logger.Info("Rewriting Container image",
			"namespace", pod.Namespace,
			"pod", pod.Name,
			"original-image", container.Image,
			"mutated-image", newImage,
		)
	}

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
