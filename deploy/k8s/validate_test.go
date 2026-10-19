package k8s

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKubernetesManifests_PureASCIIAndExistence(t *testing.T) {
	requiredFiles := []string{
		"base/namespace.yaml",
		"base/configmap.yaml",
		"base/secret.yaml",
		"base/api-deployment.yaml",
		"base/api-service.yaml",
		"base/api-hpa.yaml",
		"base/api-pdb.yaml",
		"base/ingress.yaml",
		"base/kustomization.yaml",
	}

	for _, rel := range requiredFiles {
		p := filepath.Join(".", rel)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("expected file %s to exist: %v", rel, err)
		}

		// Ensure 100% pure 7-bit ASCII
		for i, b := range data {
			if b > 127 {
				t.Fatalf("file %s contains non-ASCII byte at index %d: 0x%X", rel, i, b)
			}
		}

		content := string(data)
		if strings.Contains(rel, "hpa") {
			if !strings.Contains(content, "HorizontalPodAutoscaler") || !strings.Contains(content, "autoscaling/v2") {
				t.Fatalf("HPA manifest must use autoscaling/v2 and HorizontalPodAutoscaler")
			}
			if !strings.Contains(content, "maxReplicas: 100") {
				t.Fatalf("HPA manifest must allow scaling up to 100 replicas for 1M RPS bursts")
			}
		}
		if strings.Contains(rel, "pdb") {
			if !strings.Contains(content, "PodDisruptionBudget") || !strings.Contains(content, "minAvailable") {
				t.Fatalf("PDB manifest must define PodDisruptionBudget with minAvailable")
			}
		}
	}
}

func TestKubernetesManifests_ProductionWiring(t *testing.T) {
	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(".", rel))
		if err != nil {
			t.Fatalf("expected file %s to exist: %v", rel, err)
		}
		return string(data)
	}

	configMap := read("base/configmap.yaml")
	for _, key := range []string{"MINIO_ENDPOINT", "MINIO_BUCKET", "API_DOMAIN", "WEBSITE_DOMAIN", "CORS_ALLOWED_ORIGINS"} {
		if !strings.Contains(configMap, key) {
			t.Fatalf("configmap must define %s", key)
		}
	}
	if strings.Contains(configMap, "$(REDIS_PASSWORD)") {
		t.Fatalf("configmap must not rely on unexpanded $(REDIS_PASSWORD)")
	}

	secret := read("base/secret.yaml")
	for _, key := range []string{"MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "REDIS_URL"} {
		if !strings.Contains(secret, key) {
			t.Fatalf("secret must define %s", key)
		}
	}

	deployment := read("base/api-deployment.yaml")
	if !strings.Contains(deployment, "tcpSocket") {
		t.Fatalf("deployment liveness probe must use tcpSocket")
	}

	ingress := read("base/ingress.yaml")
	if !strings.Contains(ingress, "tls:") {
		t.Fatalf("ingress must define a tls block")
	}
}
