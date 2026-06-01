package kubernetes

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bryanbarton525/prism/internal/plugins"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type fakeClient struct{}

func (fakeClient) ServerVersion(context.Context) (string, error) {
	return "v1.35.2", nil
}

func (fakeClient) Namespaces(context.Context) ([]corev1.Namespace, error) {
	return []corev1.Namespace{{
		ObjectMeta: metav1.ObjectMeta{Name: "temporal", CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour))},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}}, nil
}

func (fakeClient) Pods(context.Context, string) ([]corev1.Pod, error) {
	return []corev1.Pod{{
		ObjectMeta: metav1.ObjectMeta{Name: "temporal-frontend-abc", CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour))},
		Spec:       corev1.PodSpec{NodeName: "node-a"},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			PodIP: "10.42.0.10",
			InitContainerStatuses: []corev1.ContainerStatus{
				{Name: "check-cassandra", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}},
				{Name: "check-elasticsearch-index", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			},
			ContainerStatuses: []corev1.ContainerStatus{{Name: "temporal-frontend", Ready: false}},
		},
	}}, nil
}

func (fakeClient) Deployments(context.Context, string) ([]appsv1.Deployment, error) {
	one := int32(1)
	return []appsv1.Deployment{{
		ObjectMeta: metav1.ObjectMeta{Name: "temporal-frontend", CreationTimestamp: metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))},
		Spec:       appsv1.DeploymentSpec{Replicas: &one},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 0, UpdatedReplicas: 1, AvailableReplicas: 0},
	}}, nil
}

func (fakeClient) ReplicaSets(context.Context, string) ([]appsv1.ReplicaSet, error) {
	one := int32(1)
	return []appsv1.ReplicaSet{{
		ObjectMeta: metav1.ObjectMeta{Name: "temporal-frontend-abc", CreationTimestamp: metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))},
		Spec:       appsv1.ReplicaSetSpec{Replicas: &one},
		Status:     appsv1.ReplicaSetStatus{Replicas: 1, ReadyReplicas: 0},
	}}, nil
}

func (fakeClient) Services(context.Context, string) ([]corev1.Service, error) {
	return []corev1.Service{{
		ObjectMeta: metav1.ObjectMeta{Name: "temporal-frontend", CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * 24 * time.Hour))},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.43.0.10",
			Ports:     []corev1.ServicePort{{Port: 7233, Protocol: corev1.ProtocolTCP}},
		},
	}}, nil
}

func (fakeClient) Events(context.Context, string) ([]corev1.Event, error) {
	return []corev1.Event{{
		ObjectMeta:     metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute))},
		LastTimestamp:  metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		Type:           corev1.EventTypeWarning,
		Reason:         "Unhealthy",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "elasticsearch-master-0"},
		Message:        "Readiness probe failed: waiting for elasticsearch cluster",
	}}, nil
}

func (fakeClient) EndpointSlices(context.Context, string) ([]discoveryv1.EndpointSlice, error) {
	port := int32(7233)
	protocol := corev1.ProtocolTCP
	return []discoveryv1.EndpointSlice{{
		ObjectMeta:  metav1.ObjectMeta{Name: "temporal-frontend", CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * 24 * time.Hour))},
		AddressType: discoveryv1.AddressTypeIPv4,
		Ports:       []discoveryv1.EndpointPort{{Port: &port, Protocol: &protocol}},
		Endpoints:   []discoveryv1.Endpoint{{Addresses: []string{"10.42.0.10"}}},
	}}, nil
}

func (fakeClient) HTTPRoutes(context.Context, string) ([]string, error) {
	return []string{"temporal-web-ui-route\t[temporal.barton.local]\t2025-10-11T23:39:18Z"}, nil
}

func TestPluginCollectDiagnosticsUsesNativeClient(t *testing.T) {
	plugin := NewWithClient(fakeClient{})
	res, err := plugin.Call(context.Background(), plugins.ToolCall{
		Tool: ToolCollectDiagnostics,
		Args: map[string]string{"namespace": "temporal"},
	})
	if err != nil {
		t.Fatalf("Call(): %v", err)
	}

	for _, want := range []string{
		"$ kubernetes.server_version",
		"v1.35.2",
		"$ kubernetes.list_pods namespace=temporal",
		"temporal-frontend-abc\t0/1\tInit:1/2",
		"Readiness probe failed: waiting for elasticsearch cluster",
		"temporal-web-ui-route",
	} {
		if !strings.Contains(res.Content, want) {
			t.Fatalf("result missing %q:\n%s", want, res.Content)
		}
	}
}
