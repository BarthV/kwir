package kwir

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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
func (cfg *kwirConfig) rewriteImage(image string) (string, error) {
	newImage := image

	for _, rule := range cfg.RewriteRules.PrefixRules {
		changed := false
		newImage, changed = rule.applyPrefixRule(newImage)
		if changed && cfg.RewritePolicy == stopAfterFirstMatchPolicy {
			return newImage, nil
		}
	}

	for _, rule := range cfg.RewriteRules.RegexRules {
		changed := false
		newImage, changed = rule.applyRegexRule(newImage)
		if changed && cfg.RewritePolicy == stopAfterFirstMatchPolicy {
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
	patches := []webhook.JSONPatchOp{}

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

	// If the pod doesnt have annotations prepend a patch
	// so the annotations map exists before the patches above
	if pod.Annotations == nil {
		patches = append(patches, webhook.JSONPatchOp{
			Operation: "add",
			Path:      "/metadata/annotations",
			Value:     map[string]string{},
		})
	}

	patches = append(patches, webhook.JSONPatchOp{
		Operation: "add",
		Path:      "/metadata/annotations/kwir-podrewriter-modified",
		Value:     "true",
	})

	// rewrite any existing InitContainers
	for i, container := range pod.Spec.InitContainers {
		newImage, err := a.cfg.rewriteImage(container.Image)

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

		patches = append(patches, webhook.JSONPatchOp{
			Operation: "replace",
			Path:      fmt.Sprintf("/spec/initContainers/%d/image", i),
			Value:     newImage,
		})
	}

	// rewrite any existing Containers
	for i, container := range pod.Spec.Containers {
		newImage, err := a.cfg.rewriteImage(container.Image)

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

			patches = append(patches, webhook.JSONPatchOp{
				Operation: "replace",
				Path:      fmt.Sprintf("/spec/containers/%d/image", i),
				Value:     newImage,
			})

		} else {
			logger.Info("Image kept unchanged",
				"namespace", pod.Namespace,
				"pod", pod.Name,
				"image", container.Image,
			)
		}
	}

	// apply pod mutation & reply to admission request
	return admission.Patched("Pod images rewritten", patches...)
}

// PodRewriter implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (a *PodRewriter) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
