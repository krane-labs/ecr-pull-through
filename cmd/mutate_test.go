package main

import (
	"encoding/json"
	"testing"

	v1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestActuallyMutate(t *testing.T) {
	// set test config
	config = &Config{
		Registries:   []string{"ghcr.io", "docker.io"},
		AwsAccountID: "12345",
		AwsRegion:    "us-west-2",
	}

	// build a pod with various image forms
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "c-nginx", Image: "nginx"},
				{Name: "c-ghcr", Image: "ghcr.io/owner/image:tag"},
			},
			InitContainers: []corev1.Container{
				{Name: "init-1", Image: "owner/init:1.0"},
			},
			EphemeralContainers: []corev1.EphemeralContainer{
				{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "e-1", Image: "quay.io/org/repo:tag"}},
			},
		},
	}

	podJSON, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("marshal pod: %v", err)
	}

	admReq := &v1beta1.AdmissionRequest{
		UID: "test-uid",
		Object: runtime.RawExtension{
			Raw: podJSON,
		},
	}

	admReview := &v1beta1.AdmissionReview{Request: admReq}

	body, err := json.Marshal(admReview)
	if err != nil {
		t.Fatalf("marshal admissionreview: %v", err)
	}

	mutated, err := actuallyMutate(body)
	if err != nil {
		t.Fatalf("actuallyMutate error: %v", err)
	}

	out := v1beta1.AdmissionReview{}
	if err := json.Unmarshal(mutated, &out); err != nil {
		t.Fatalf("unmarshal mutated review: %v", err)
	}

	if out.Response == nil {
		t.Fatalf("response is nil")
	}

	var patches []map[string]string
	if err := json.Unmarshal(out.Response.Patch, &patches); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}

	// helper to find patch by path
	find := func(path string) (string, bool) {
		for _, p := range patches {
			if v, ok := p["path"]; ok && v == path {
				return p["value"], true
			}
		}
		return "", false
	}

	// expected patches
	wantC0 := "12345.dkr.ecr.us-west-2.amazonaws.com/docker.io/library/nginx"
	if got, ok := find("/spec/containers/0/image"); !ok {
		t.Fatalf("missing patch for containers/0")
	} else if got != wantC0 {
		t.Fatalf("containers/0: got %q want %q", got, wantC0)
	}

	wantC1 := "12345.dkr.ecr.us-west-2.amazonaws.com/ghcr.io/owner/image:tag"
	if got, ok := find("/spec/containers/1/image"); !ok {
		t.Fatalf("missing patch for containers/1")
	} else if got != wantC1 {
		t.Fatalf("containers/1: got %q want %q", got, wantC1)
	}

	wantInit0 := "12345.dkr.ecr.us-west-2.amazonaws.com/docker.io/owner/init:1.0"
	if got, ok := find("/spec/initContainers/0/image"); !ok {
		t.Fatalf("missing patch for initContainers/0")
	} else if got != wantInit0 {
		t.Fatalf("initContainers/0: got %q want %q", got, wantInit0)
	}

	// ephemeral container should not have a patch because quay.io is not in registries
	if _, ok := find("/spec/ephemeralContainers/0/image"); ok {
		t.Fatalf("unexpected patch for ephemeralContainers/0")
	}
}
