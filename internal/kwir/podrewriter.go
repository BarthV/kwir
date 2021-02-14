package kwir

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

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

func (rl *rule) applyPrefixRule(image string) (string, bool) {
	if !strings.HasPrefix(image, rl.Match) {
		return image, false
	}
	return rl.Replace + image[len(rl.Match):], true
}

func (rl *rule) applyRegexRule(image string) (string, bool) {
	if !rl.regex.MatchString(image) {
		return image, false
	}
	return rl.regex.ReplaceAllString(image, rl.Replace), true
}

// rewriteImages takes an input string and applies (in order) all rewrite rules then returns it
func (a *PodRewriter) rewriteImage(image string) (string, error) {
	newImage := image

	for _, rule := range a.cfg.RewriteRules.PrefixRules {
		changed := false
		newImage, changed = rule.applyPrefixRule(newImage)
		if changed && a.cfg.RewritePolicy == stopAfterFirstMatchPolicy {
			return newImage, nil
		}
	}

	for _, rule := range a.cfg.RewriteRules.RegexRules {
		changed := false
		newImage, changed = rule.applyRegexRule(newImage)
		if changed && a.cfg.RewritePolicy == stopAfterFirstMatchPolicy {
			return newImage, nil
		}
	}

	return newImage, nil
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

// Handle is a kube admission webhook handler that rewrite Pod containers images based on its own config rules
func (a *PodRewriter) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.Log.WithName("kwir-podrewriter")
	pod := &corev1.Pod{}

	err := a.decoder.Decode(req, pod)

	// Don't modify and allow admission if received object cannot be parsed as a core/v1/Pod
	if err != nil {
		logger.Info("Webhook request is not a v1/Pod",
			"kind", req.Kind,
			"name", req.Name,
			"namespace", req.Namespace,
		)
		return admission.Allowed("Unsupported API object: No change applied")
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

		if newImage != container.Image {
			logger.Info("Rewriting Container image",
				"namespace", pod.Namespace,
				"pod", pod.Name,
				"original-image", container.Image,
				"mutated-image", newImage,
			)
		} else {
			logger.Info("Image kept unchanged",
				"namespace", pod.Namespace,
				"pod", pod.Name,
				"image", container.Image,
			)
		}
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
