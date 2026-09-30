package kubectl_test

import (
	"os"
	"strings"
	"testing"

	"github.com/prunesh/kubectl/filter"
)

// --- Rewrite ---

func TestRewriteLogsAddsTail(t *testing.T) {
	got, ok := filter.Rewrite([]string{"logs", "my-pod"})
	if !ok {
		t.Fatal("expected rewrite for logs")
	}
	var found bool
	for _, a := range got {
		if strings.HasPrefix(a, "--tail=") {
			found = true
		}
	}
	if !found {
		t.Fatalf("--tail flag not injected: %v", got)
	}
}

func TestRewriteLogsNoRewriteWithFollow(t *testing.T) {
	_, ok := filter.Rewrite([]string{"logs", "-f", "my-pod"})
	if ok {
		t.Fatal("must not rewrite logs with -f")
	}
}

func TestRewriteLogsNoRewriteWithTail(t *testing.T) {
	_, ok := filter.Rewrite([]string{"logs", "--tail=200", "my-pod"})
	if ok {
		t.Fatal("must not rewrite logs when --tail is already set")
	}
}

func TestRewriteGetNoRewrite(t *testing.T) {
	_, ok := filter.Rewrite([]string{"get", "pods"})
	if ok {
		t.Fatal("must not rewrite get")
	}
}

func TestRewriteDescribeNoRewrite(t *testing.T) {
	_, ok := filter.Rewrite([]string{"describe", "pod/my-pod"})
	if ok {
		t.Fatal("must not rewrite describe")
	}
}

func TestRewriteLogsWithNamespaceFlag(t *testing.T) {
	got, ok := filter.Rewrite([]string{"-n", "my-ns", "logs", "my-pod"})
	if !ok {
		t.Fatal("expected rewrite for logs with -n flag")
	}
	var found bool
	for _, a := range got {
		if strings.HasPrefix(a, "--tail=") {
			found = true
		}
	}
	if !found {
		t.Fatalf("--tail not injected: %v", got)
	}
}

// --- FilterOutput: describe ---

const describePodOutput = `Name:         my-pod-abc123
Namespace:    default
Priority:     0
Service Account:  default
Node:         node1/10.0.0.1
Start Time:   Fri, 29 Aug 2026 10:00:00 +0000
Labels:       app=my-app
Annotations:  kubectl.kubernetes.io/last-applied-configuration: {"apiVersion":"v1","kind":"Pod","metadata":{"annotations":{},"name":"my-pod","namespace":"default"},"spec":{"containers":[{"image":"my-image:latest","name":"my-container"}]}}
Status:       Running
IP:           10.244.0.5
Containers:
  my-container:
    Container ID:   containerd://abc123def456789
    Image:          my-image:latest
    Image ID:       docker-pullable://my-image@sha256:abc123
    Port:           8080/TCP
    Host Port:      0/TCP
    State:          Running
      Started:      Fri, 29 Aug 2026 10:01:00 +0000
    Ready:          True
    Restart Count:  0
    Mounts:
      /var/run/secrets/kubernetes.io/serviceaccount from kube-api-access (ro)
Conditions:
  Type              Status
  Initialized       True
  Ready             True
Events:
  Type    Reason     Age   From               Message
  ----    ------     ---   ----               -------
  Normal  Scheduled  2m    default-scheduler  Successfully assigned
Volumes:
  kube-api-access:
    Type:  Projected
    TokenExpirationSeconds:  3607
QoS Class:         BestEffort
Node-Selectors:    <none>
Tolerations:       node.kubernetes.io/not-ready:NoExecute op=Exists for 300s
                   node.kubernetes.io/unreachable:NoExecute op=Exists for 300s
`

func TestFilterDescribeStripsAnnotations(t *testing.T) {
	out := filter.FilterOutput([]string{"describe", "pod/my-pod"}, describePodOutput, 0)
	if strings.Contains(out, "Annotations:") {
		t.Error("filtered describe must not contain Annotations section")
	}
	if strings.Contains(out, "last-applied-configuration") {
		t.Error("filtered describe must not contain annotation content")
	}
}

func TestFilterDescribeStripsVolumes(t *testing.T) {
	out := filter.FilterOutput([]string{"describe", "pod/my-pod"}, describePodOutput, 0)
	if strings.Contains(out, "Volumes:") {
		t.Error("filtered describe must not contain Volumes section")
	}
}

func TestFilterDescribeStripsTolerations(t *testing.T) {
	out := filter.FilterOutput([]string{"describe", "pod/my-pod"}, describePodOutput, 0)
	if strings.Contains(out, "Tolerations:") {
		t.Error("filtered describe must not contain Tolerations section")
	}
}

func TestFilterDescribeStripsContainerID(t *testing.T) {
	out := filter.FilterOutput([]string{"describe", "pod/my-pod"}, describePodOutput, 0)
	if strings.Contains(out, "Container ID:") {
		t.Error("filtered describe must not contain Container ID")
	}
	if strings.Contains(out, "Image ID:") {
		t.Error("filtered describe must not contain Image ID")
	}
}

func TestFilterDescribeKeepsEssential(t *testing.T) {
	out := filter.FilterOutput([]string{"describe", "pod/my-pod"}, describePodOutput, 0)
	for _, want := range []string{"Name:", "Status:", "Containers:", "Events:", "Conditions:"} {
		if !strings.Contains(out, want) {
			t.Errorf("filtered describe must contain %q", want)
		}
	}
}

func TestFilterDescribeKeepsImage(t *testing.T) {
	out := filter.FilterOutput([]string{"describe", "pod/my-pod"}, describePodOutput, 0)
	if !strings.Contains(out, "Image:") {
		t.Error("filtered describe must keep Image field inside Containers")
	}
}

func TestFilterDescribeIsSmaller(t *testing.T) {
	out := filter.FilterOutput([]string{"describe", "pod/my-pod"}, describePodOutput, 0)
	if len(out) >= len(describePodOutput) {
		t.Errorf("filtered describe (%d bytes) must be smaller than original (%d bytes)", len(out), len(describePodOutput))
	}
}

// --- FilterOutput: get -o yaml ---

const getYAMLOutput = `apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2026-08-29T10:00:00Z"
  managedFields:
  - apiVersion: v1
    fieldsType: FieldsV1
    fieldsV1:
      f:spec:
        f:containers:
          k:{"name":"my-container"}:
            .: {}
    manager: kubectl
    operation: Apply
  name: my-pod
  namespace: default
  resourceVersion: "12345"
  uid: abc123-def456
spec:
  containers:
  - image: my-image:latest
    name: my-container
status:
  phase: Running
`

func TestFilterYAMLStripsManagedFields(t *testing.T) {
	out := filter.FilterOutput([]string{"get", "pod/my-pod", "-o", "yaml"}, getYAMLOutput, 0)
	if strings.Contains(out, "managedFields") {
		t.Error("filtered yaml must not contain managedFields")
	}
}

func TestFilterYAMLStripsMetadataNoise(t *testing.T) {
	out := filter.FilterOutput([]string{"get", "pod/my-pod", "-o", "yaml"}, getYAMLOutput, 0)
	for _, noisy := range []string{"resourceVersion", "uid", "creationTimestamp"} {
		if strings.Contains(out, noisy) {
			t.Errorf("filtered yaml must not contain %q", noisy)
		}
	}
}

func TestFilterYAMLKeepsSpec(t *testing.T) {
	out := filter.FilterOutput([]string{"get", "pod/my-pod", "-o", "yaml"}, getYAMLOutput, 0)
	if !strings.Contains(out, "spec:") || !strings.Contains(out, "my-image:latest") {
		t.Error("filtered yaml must keep spec and container image")
	}
}

func TestFilterYAMLIsSmaller(t *testing.T) {
	out := filter.FilterOutput([]string{"get", "pod/my-pod", "-o", "yaml"}, getYAMLOutput, 0)
	if len(out) >= len(getYAMLOutput) {
		t.Errorf("filtered yaml (%d bytes) must be smaller than original (%d bytes)", len(out), len(getYAMLOutput))
	}
}

// --- FilterOutput: get (table, no -o flag) ---

func TestFilterGetTablePassthrough(t *testing.T) {
	input := "NAME       READY   STATUS    RESTARTS   AGE\nmy-pod     1/1     Running   0          2m\n"
	out := filter.FilterOutput([]string{"get", "pods"}, input, 0)
	if out != input {
		t.Errorf("table get must pass through unchanged")
	}
}

// --- FilterOutput: passthrough subcommands ---

func TestFilterApplyPassthrough(t *testing.T) {
	input := "deployment.apps/my-app configured\n"
	out := filter.FilterOutput([]string{"apply", "-f", "deploy.yaml"}, input, 0)
	if out != input {
		t.Errorf("apply must pass through unchanged")
	}
}

func TestFilterDeletePassthrough(t *testing.T) {
	input := "pod \"my-pod\" deleted\n"
	out := filter.FilterOutput([]string{"delete", "pod/my-pod"}, input, 0)
	if out != input {
		t.Errorf("delete must pass through unchanged")
	}
}

// --- ID constant ---

func TestID(t *testing.T) {
	if filter.ID != "prunesh/kubectl" {
		t.Fatalf("ID %q does not follow author/<cmd> rule", filter.ID)
	}
}

// --- prunesh.toml manifest ---

func TestManifest(t *testing.T) {
	data, err := os.ReadFile("prunesh.toml")
	if err != nil {
		t.Fatalf("read prunesh.toml: %v", err)
	}
	manifest := string(data)
	for _, want := range []string{
		`id = "prunesh/kubectl"`,
		`command = "kubectl"`,
		`platforms = [`,
		`contract = "stdin/v1"`,
		`[prunesh-core-version]`,
		`version = "0.16.0"`,
		`constraint = "min"`,
	} {
		if !strings.Contains(manifest, want) {
			t.Errorf("manifest must contain %q", want)
		}
	}
}
