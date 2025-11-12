package main

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "testing"

    v1beta1 "k8s.io/api/admission/v1beta1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
)

func TestMutateHandlerIntegration(t *testing.T) {
    // Set test config
    config = &Config{
        Registries:   []string{"ghcr.io", "docker.io"},
        AwsAccountID: "99999",
        AwsRegion:    "eu-central-1",
    }

    // Build a pod similar to real usage
    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{Name: "int-pod", Namespace: "default"},
        Spec: corev1.PodSpec{
            Containers: []corev1.Container{
                {Name: "c1", Image: "nginx:latest"},
                {Name: "c2", Image: "ghcr.io/owner/app:1.0"},
            },
            InitContainers: []corev1.Container{{Name: "init1", Image: "owner/init:0.1"}},
            EphemeralContainers: []corev1.EphemeralContainer{{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "e1", Image: "quay.io/org/repo:tag"}}},
        },
    }

    podJSON, err := json.Marshal(pod)
    if err != nil {
        t.Fatalf("marshal pod: %v", err)
    }

    admReq := &v1beta1.AdmissionRequest{UID: "u1", Object: runtimeRaw(podJSON)}
    admReview := &v1beta1.AdmissionReview{Request: admReq}
    body, err := json.Marshal(admReview)
    if err != nil {
        t.Fatalf("marshal review: %v", err)
    }

    // Start test server with handlers
    mux := http.NewServeMux()
    mux.HandleFunc("/mutate", handleMutate)
    srv := httptest.NewServer(mux)
    defer srv.Close()

    resp, err := http.Post(srv.URL+"/mutate", "application/json", bytes.NewReader(body))
    if err != nil {
        t.Fatalf("post mutate: %v", err)
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        t.Fatalf("read resp: %v", err)
    }

    out := v1beta1.AdmissionReview{}
    if err := json.Unmarshal(respBody, &out); err != nil {
        t.Fatalf("unmarshal resp: %v", err)
    }

    if out.Response == nil {
        t.Fatalf("nil response")
    }

    var patches []map[string]string
    if err := json.Unmarshal(out.Response.Patch, &patches); err != nil {
        t.Fatalf("unmarshal patch: %v", err)
    }

    // helper
    find := func(path string) (string, bool) {
        for _, p := range patches {
            if p["path"] == path {
                return p["value"], true
            }
        }
        return "", false
    }

    want0 := "99999.dkr.ecr.eu-central-1.amazonaws.com/docker.io/library/nginx:latest"
    if got, ok := find("/spec/containers/0/image"); !ok {
        t.Fatalf("missing patch for containers/0")
    } else if got != want0 {
        t.Fatalf("containers/0 got=%q want=%q", got, want0)
    }

    want1 := "99999.dkr.ecr.eu-central-1.amazonaws.com/ghcr.io/owner/app:1.0"
    if got, ok := find("/spec/containers/1/image"); !ok {
        t.Fatalf("missing patch for containers/1")
    } else if got != want1 {
        t.Fatalf("containers/1 got=%q want=%q", got, want1)
    }

    wantInit := "99999.dkr.ecr.eu-central-1.amazonaws.com/docker.io/owner/init:0.1"
    if got, ok := find("/spec/initContainers/0/image"); !ok {
        t.Fatalf("missing patch for initContainers/0")
    } else if got != wantInit {
        t.Fatalf("initContainers/0 got=%q want=%q", got, wantInit)
    }

    if _, ok := find("/spec/ephemeralContainers/0/image"); ok {
        t.Fatalf("ephemeral patch found but should not be present")
    }
}

// runtimeRaw is a tiny helper to build a RawExtension from bytes without
// importing extra runtime packages into this test file.
func runtimeRaw(b []byte) runtime.RawExtension {
    return runtime.RawExtension{Raw: b}
}
