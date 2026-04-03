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
	config = &Config{
		Registries:   []string{"ghcr.io", "docker.io"},
		AwsAccountID: "12345",
		AwsRegion:    "us-west-2",
	}

	t.Run("containers", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "c-nginx", Image: "nginx"},
					{Name: "c-ghcr", Image: "ghcr.io/owner/image:tag"},
					{Name: "c-registry1", Image: "registry-1.docker.io/kranelabs/redis:8.6.1"},
					{Name: "c-official", Image: "registry-1.docker.io/redis"},
				},
			},
		}
		checkMutatePatch(t, pod, map[string]string{
			"/spec/containers/0/image": "12345.dkr.ecr.us-west-2.amazonaws.com/docker.io/library/nginx",
			"/spec/containers/1/image": "12345.dkr.ecr.us-west-2.amazonaws.com/ghcr.io/owner/image:tag",
			"/spec/containers/2/image": "12345.dkr.ecr.us-west-2.amazonaws.com/docker.io/kranelabs/redis:8.6.1",
			"/spec/containers/3/image": "12345.dkr.ecr.us-west-2.amazonaws.com/docker.io/library/redis",
		})
	})

	t.Run("initContainers", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "init-1", Image: "owner/init:1.0"},
				},
			},
		}
		checkMutatePatch(t, pod, map[string]string{
			"/spec/initContainers/0/image": "12345.dkr.ecr.us-west-2.amazonaws.com/docker.io/owner/init:1.0",
		})
	})

	t.Run("ephemeralContainers not patched", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
			Spec: corev1.PodSpec{
				EphemeralContainers: []corev1.EphemeralContainer{
					{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "e-1", Image: "quay.io/org/repo:tag"}},
				},
			},
		}
		checkMutatePatch(t, pod, map[string]string{}) // expect no patch
	})
}

func checkMutatePatch(t *testing.T, pod *corev1.Pod, want map[string]string) {
	t.Helper()
	podJSON, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("marshal pod: %v", err)
	}
	admReq := &v1beta1.AdmissionRequest{
		UID:    "test-uid",
		Object: runtime.RawExtension{Raw: podJSON},
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
	got := map[string]string{}
	for _, p := range patches {
		if path, ok := p["path"]; ok {
			got[path] = p["value"]
		}
	}
	for k, v := range want {
		if gotV, ok := got[k]; !ok {
			t.Errorf("missing patch for %s", k)
		} else if gotV != v {
			t.Errorf("patch for %s: got %q want %q", k, gotV, v)
		}
	}
	// Check for unexpected patches
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected patch for %s", k)
		}
	}
}
