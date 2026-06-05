package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/plugins"
	"github.com/bryanbarton525/prism/pkg/evidence"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	ToolCollectDiagnostics = "kubernetes.collect_diagnostics"
	outputLimit            = 24000
)

type Client interface {
	ServerVersion(ctx context.Context) (string, error)
	Namespaces(ctx context.Context) ([]corev1.Namespace, error)
	Pods(ctx context.Context, namespace string) ([]corev1.Pod, error)
	Deployments(ctx context.Context, namespace string) ([]appsv1.Deployment, error)
	ReplicaSets(ctx context.Context, namespace string) ([]appsv1.ReplicaSet, error)
	Services(ctx context.Context, namespace string) ([]corev1.Service, error)
	Events(ctx context.Context, namespace string) ([]corev1.Event, error)
	EndpointSlices(ctx context.Context, namespace string) ([]discoveryv1.EndpointSlice, error)
	HTTPRoutes(ctx context.Context, namespace string) ([]string, error)
}

// Plugin collects read-only Kubernetes evidence through native client-go clients.
type Plugin struct {
	clientFactory func() (Client, error)
}

func New() *Plugin {
	return &Plugin{clientFactory: defaultClient}
}

func NewWithClient(client Client) *Plugin {
	return &Plugin{clientFactory: func() (Client, error) { return client, nil }}
}

func (p *Plugin) Name() string {
	return "kubernetes"
}

func (p *Plugin) Tools() []plugins.ToolSpec {
	return []plugins.ToolSpec{{
		Name:        ToolCollectDiagnostics,
		Description: "Collect read-only Kubernetes namespace/workload diagnostics.",
		ReadOnly:    true,
		Mode:        "read_only",
		MaxBytes:    outputLimit,
	}}
}

func (p *Plugin) Call(ctx context.Context, call plugins.ToolCall) (plugins.ToolResult, error) {
	switch call.Tool {
	case ToolCollectDiagnostics:
		client, err := p.clientFactory()
		if err != nil {
			return plugins.ToolResult{}, err
		}
		return collectDiagnostics(ctx, client, call.Args), nil
	default:
		return plugins.ToolResult{}, fmt.Errorf("unsupported Kubernetes tool %q", call.Tool)
	}
}

type realClient struct {
	typed   kubernetes.Interface
	dynamic dynamic.Interface
	rest    *rest.Config
}

func defaultClient() (Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	typed, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &realClient{typed: typed, dynamic: dyn, rest: cfg}, nil
}

func loadConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err == nil {
		return cfg, nil
	}
	inCluster, inClusterErr := rest.InClusterConfig()
	if inClusterErr == nil {
		return inCluster, nil
	}
	return nil, err
}

func (c *realClient) ServerVersion(ctx context.Context) (string, error) {
	version, err := c.typed.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return version.String(), nil
}

func (c *realClient) Namespaces(ctx context.Context) ([]corev1.Namespace, error) {
	list, err := c.typed.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *realClient) Pods(ctx context.Context, namespace string) ([]corev1.Pod, error) {
	list, err := c.typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *realClient) Deployments(ctx context.Context, namespace string) ([]appsv1.Deployment, error) {
	list, err := c.typed.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *realClient) ReplicaSets(ctx context.Context, namespace string) ([]appsv1.ReplicaSet, error) {
	list, err := c.typed.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *realClient) Services(ctx context.Context, namespace string) ([]corev1.Service, error) {
	list, err := c.typed.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *realClient) Events(ctx context.Context, namespace string) ([]corev1.Event, error) {
	list, err := c.typed.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *realClient) EndpointSlices(ctx context.Context, namespace string) ([]discoveryv1.EndpointSlice, error) {
	list, err := c.typed.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *realClient) HTTPRoutes(ctx context.Context, namespace string) ([]string, error) {
	gvr := schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "httproutes",
	}
	list, err := c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		hosts, _, _ := unstructuredSlice(item.Object, "spec", "hostnames")
		out = append(out, fmt.Sprintf("%s\t%v\t%s", item.GetName(), hosts, item.GetCreationTimestamp().Time.Format(time.RFC3339)))
	}
	return out, nil
}

func collectDiagnostics(ctx context.Context, client Client, args map[string]string) plugins.ToolResult {
	namespace := args["namespace"]
	var b strings.Builder

	writeSection(&b, "kubernetes.server_version", func() string {
		version, err := client.ServerVersion(ctx)
		if err != nil {
			return formatError(err)
		}
		return version
	})

	writeSection(&b, "kubernetes.list_namespaces", func() string {
		items, err := client.Namespaces(ctx)
		if err != nil {
			return formatError(err)
		}
		return formatNamespaces(items)
	})

	if namespace != "" {
		writeSection(&b, "kubernetes.list_pods namespace="+namespace, func() string {
			items, err := client.Pods(ctx, namespace)
			if err != nil {
				return formatError(err)
			}
			return formatPods(items)
		})
		writeSection(&b, "kubernetes.list_deployments namespace="+namespace, func() string {
			items, err := client.Deployments(ctx, namespace)
			if err != nil {
				return formatError(err)
			}
			return formatDeployments(items)
		})
		writeSection(&b, "kubernetes.list_replicasets namespace="+namespace, func() string {
			items, err := client.ReplicaSets(ctx, namespace)
			if err != nil {
				return formatError(err)
			}
			return formatReplicaSets(items)
		})
		writeSection(&b, "kubernetes.list_services namespace="+namespace, func() string {
			items, err := client.Services(ctx, namespace)
			if err != nil {
				return formatError(err)
			}
			return formatServices(items)
		})
		writeSection(&b, "kubernetes.list_events namespace="+namespace, func() string {
			items, err := client.Events(ctx, namespace)
			if err != nil {
				return formatError(err)
			}
			return formatEvents(items)
		})
		writeSection(&b, "kubernetes.list_endpoint_slices namespace="+namespace, func() string {
			items, err := client.EndpointSlices(ctx, namespace)
			if err != nil {
				return formatError(err)
			}
			return formatEndpointSlices(items)
		})
		writeSection(&b, "kubernetes.list_http_routes namespace="+namespace, func() string {
			items, err := client.HTTPRoutes(ctx, namespace)
			if err != nil {
				return formatError(err)
			}
			return strings.Join(items, "\n")
		})
	}

	content := truncate(strings.TrimSpace(b.String()), outputLimit)
	pack := evidence.Pack{
		Kind:           "kubernetes.diagnostics",
		Source:         namespace,
		Plugin:         "kubernetes",
		CollectionTime: time.Now().UTC(),
		Limits: evidence.Limits{
			MaxBytes:  outputLimit,
			MaxEvents: 100,
		},
		Summary: map[string]any{
			"namespace": namespace,
			"bounded":   true,
		},
		Artifacts: []evidence.Artifact{{
			Type:    "diagnostics_text",
			Name:    "kubectl-equivalent-summary",
			Content: content,
		}},
	}
	if namespace == "" {
		pack.Errors = append(pack.Errors, "namespace not inferred; collected cluster-level evidence only")
	}
	return plugins.ToolResult{
		Label:        "runtime-plugin:kubernetes",
		Content:      content,
		EvidencePack: &pack,
	}
}

func marshalEvidencePack(pack *evidence.Pack) string {
	if pack == nil {
		return ""
	}
	data, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

func writeSection(b *strings.Builder, title string, fn func() string) {
	b.WriteString("$ ")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(fn()))
	b.WriteString("\n\n")
}

func formatError(err error) string {
	return "[error] " + err.Error()
}

func formatNamespaces(items []corev1.Namespace) string {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	var b strings.Builder
	b.WriteString("NAME\tSTATUS\tAGE\n")
	for _, item := range items {
		b.WriteString(fmt.Sprintf("%s\t%s\t%s\n", item.Name, item.Status.Phase, age(item.CreationTimestamp.Time)))
	}
	return b.String()
}

func formatPods(items []corev1.Pod) string {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	var b strings.Builder
	b.WriteString("NAME\tREADY\tSTATUS\tRESTARTS\tAGE\tIP\tNODE\n")
	for _, item := range items {
		ready, total, restarts := podCounts(item)
		b.WriteString(fmt.Sprintf("%s\t%d/%d\t%s\t%d\t%s\t%s\t%s\n",
			item.Name, ready, total, podStatus(item), restarts, age(item.CreationTimestamp.Time), item.Status.PodIP, item.Spec.NodeName))
	}
	return b.String()
}

func formatDeployments(items []appsv1.Deployment) string {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	var b strings.Builder
	b.WriteString("NAME\tREADY\tUP-TO-DATE\tAVAILABLE\tAGE\n")
	for _, item := range items {
		b.WriteString(fmt.Sprintf("%s\t%d/%d\t%d\t%d\t%s\n",
			item.Name, item.Status.ReadyReplicas, replicas(item.Spec.Replicas),
			item.Status.UpdatedReplicas, item.Status.AvailableReplicas, age(item.CreationTimestamp.Time)))
	}
	return b.String()
}

func formatReplicaSets(items []appsv1.ReplicaSet) string {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	var b strings.Builder
	b.WriteString("NAME\tDESIRED\tCURRENT\tREADY\tAGE\n")
	for _, item := range items {
		b.WriteString(fmt.Sprintf("%s\t%d\t%d\t%d\t%s\n",
			item.Name, replicas(item.Spec.Replicas), item.Status.Replicas, item.Status.ReadyReplicas, age(item.CreationTimestamp.Time)))
	}
	return b.String()
}

func formatServices(items []corev1.Service) string {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	var b strings.Builder
	b.WriteString("NAME\tTYPE\tCLUSTER-IP\tPORTS\tAGE\n")
	for _, item := range items {
		b.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\t%s\n",
			item.Name, item.Spec.Type, item.Spec.ClusterIP, servicePorts(item), age(item.CreationTimestamp.Time)))
	}
	return b.String()
}

func formatEvents(items []corev1.Event) string {
	sort.SliceStable(items, func(i, j int) bool {
		return eventTime(items[i]).Before(eventTime(items[j]))
	})
	var b strings.Builder
	b.WriteString("LAST SEEN\tTYPE\tREASON\tOBJECT\tMESSAGE\n")
	start := 0
	if len(items) > 100 {
		start = len(items) - 100
	}
	for _, item := range items[start:] {
		b.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s/%s\t%s\n",
			age(eventTime(item)), item.Type, item.Reason, strings.ToLower(item.InvolvedObject.Kind), item.InvolvedObject.Name, item.Message))
	}
	return b.String()
}

func formatEndpointSlices(items []discoveryv1.EndpointSlice) string {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	var b strings.Builder
	b.WriteString("NAME\tADDRESSTYPE\tPORTS\tENDPOINTS\tAGE\n")
	for _, item := range items {
		b.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\t%s\n",
			item.Name, item.AddressType, endpointPorts(item), endpointAddresses(item), age(item.CreationTimestamp.Time)))
	}
	return b.String()
}

func podCounts(pod corev1.Pod) (ready int, total int, restarts int32) {
	total = len(pod.Status.ContainerStatuses)
	for _, status := range pod.Status.ContainerStatuses {
		if status.Ready {
			ready++
		}
		restarts += status.RestartCount
	}
	return ready, total, restarts
}

func podStatus(pod corev1.Pod) string {
	for _, status := range pod.Status.InitContainerStatuses {
		if status.State.Terminated == nil || status.State.Terminated.ExitCode != 0 {
			return fmt.Sprintf("Init:%d/%d", completedInits(pod), len(pod.Status.InitContainerStatuses))
		}
	}
	if pod.Status.Reason != "" {
		return pod.Status.Reason
	}
	return string(pod.Status.Phase)
}

func completedInits(pod corev1.Pod) int {
	var done int
	for _, status := range pod.Status.InitContainerStatuses {
		if status.State.Terminated != nil && status.State.Terminated.ExitCode == 0 {
			done++
		}
	}
	return done
}

func replicas(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func servicePorts(svc corev1.Service) string {
	ports := make([]string, 0, len(svc.Spec.Ports))
	for _, port := range svc.Spec.Ports {
		ports = append(ports, fmt.Sprintf("%d/%s", port.Port, port.Protocol))
	}
	return strings.Join(ports, ",")
}

func eventTime(event corev1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	return event.CreationTimestamp.Time
}

func endpointPorts(slice discoveryv1.EndpointSlice) string {
	ports := make([]string, 0, len(slice.Ports))
	for _, port := range slice.Ports {
		if port.Port == nil {
			continue
		}
		protocol := corev1.ProtocolTCP
		if port.Protocol != nil {
			protocol = *port.Protocol
		}
		ports = append(ports, fmt.Sprintf("%d/%s", *port.Port, protocol))
	}
	return strings.Join(ports, ",")
}

func endpointAddresses(slice discoveryv1.EndpointSlice) string {
	var addresses []string
	for _, endpoint := range slice.Endpoints {
		addresses = append(addresses, endpoint.Addresses...)
	}
	sort.Strings(addresses)
	return strings.Join(addresses, ",")
}

func age(t time.Time) string {
	if t.IsZero() {
		return "<unknown>"
	}
	d := time.Since(t).Round(time.Second)
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	default:
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n[truncated]"
}

func unstructuredSlice(obj map[string]any, fields ...string) ([]any, bool, error) {
	var current any = obj
	for _, field := range fields {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		current, ok = m[field]
		if !ok {
			return nil, false, nil
		}
	}
	values, ok := current.([]any)
	return values, ok, nil
}
