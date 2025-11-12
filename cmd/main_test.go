package main

import "testing"

func TestNormalizeDockerHubImage(t *testing.T) {
	tests := []struct {
		name string
		want string
		ok   bool
	}{
		{"nginx", "docker.io/library/nginx", true},
		{"image:1.2.3", "docker.io/library/image:1.2.3", true},
		{"owner/image", "docker.io/owner/image", true},
		{"owner/image:tag", "docker.io/owner/image:tag", true},
		{"docker.io/nginx@sha256:abc", "docker.io/library/nginx@sha256:abc", true},
		{"docker.io/library/nginx", "docker.io/library/nginx", true},
		{"docker.io/owner/image:1.2", "docker.io/owner/image:1.2", true},
		{"a/b/c:tag", "docker.io/a/b/c:tag", true},

		// non-docker registries -> not OK
		{"ghcr.io/owner/image:tag", "", false},
		{"quay.io/org/repo", "", false},
		{"registry.example.com/org/image:tag", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeDockerHubImage(tt.name)
			if ok != tt.ok {
				t.Fatalf("normalizeDockerHubImage(%q) ok = %v, want %v", tt.name, ok, tt.ok)
			}
			if tt.ok && got != tt.want {
				t.Fatalf("normalizeDockerHubImage(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
