package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"devdash/internal/model"
	"devdash/internal/logger"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type K8sMonitor struct {
	clientSet     *kubernetes.Clientset
	metricsClient *metricsv.Clientset
	config        *rest.Config
	clusters      map[string]*K8sClusterConnection
	clusterMutex  sync.RWMutex
}

type K8sClusterConnection struct {
	ID          string
	Name        string
	ClientSet   *kubernetes.Clientset
	MetricsClient *metricsv.Clientset
	Config      *rest.Config
	Status      string
	LastSync    time.Time
}

func NewK8sMonitor() (*K8sMonitor, error) {
	return &K8sMonitor{
		clusters: make(map[string]*K8sClusterConnection),
	}, nil
}

func k8sTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func (km *K8sMonitor) AddCluster(kubeconfigPath, name string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %v", err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	metricsClient, err := metricsv.NewForConfig(config)
	if err != nil {
		logger.InfoLogger(fmt.Sprintf("Metrics server not available for cluster %s: %v", name, err))
	}

	clusterID := fmt.Sprintf("%s-%d", name, time.Now().Unix())

	connection := &K8sClusterConnection{
		ID:           clusterID,
		Name:         name,
		ClientSet:    clientSet,
		MetricsClient: metricsClient,
		Config:       config,
		Status:       "connected",
		LastSync:     time.Now(),
	}

	km.clusterMutex.Lock()
	km.clusters[clusterID] = connection
	km.clusterMutex.Unlock()

	logger.InfoLogger(fmt.Sprintf("Added Kubernetes cluster: %s (%s)", name, clusterID))

	return nil
}

func (km *K8sMonitor) AddClusterWithConfig(name, apiEndpoint, caCert, token string) error {
	config := &rest.Config{
		Host:        apiEndpoint,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: []byte(caCert),
		},
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	metricsClient, err := metricsv.NewForConfig(config)
	if err != nil {
		logger.InfoLogger(fmt.Sprintf("Metrics server not available for cluster %s: %v", name, err))
	}

	clusterID := fmt.Sprintf("%s-%d", name, time.Now().Unix())

	connection := &K8sClusterConnection{
		ID:            clusterID,
		Name:          name,
		ClientSet:     clientSet,
		MetricsClient: metricsClient,
		Config:        config,
		Status:        "connected",
		LastSync:      time.Now(),
	}

	km.clusterMutex.Lock()
	km.clusters[clusterID] = connection
	km.clusterMutex.Unlock()

	return nil
}

func (km *K8sMonitor) GetCluster(clusterID string) (*K8sClusterConnection, bool) {
	km.clusterMutex.RLock()
	defer km.clusterMutex.RUnlock()

	cluster, ok := km.clusters[clusterID]
	return cluster, ok
}

func (km *K8sMonitor) ListClusters() []*model.KubernetesCluster {
	km.clusterMutex.RLock()
	defer km.clusterMutex.RUnlock()

	clusters := make([]*model.KubernetesCluster, 0, len(km.clusters))
	for _, conn := range km.clusters {
		clusterInfo, err := km.getClusterInfo(conn)
		if err != nil {
			logger.ErrorLogger(err, fmt.Sprintf("Failed to get info for cluster %s", conn.Name))
			continue
		}
		clusters = append(clusters, clusterInfo)
	}

	return clusters
}

func (km *K8sMonitor) getClusterInfo(conn *K8sClusterConnection) (*model.KubernetesCluster, error) {
	ctx, cancel := k8sTimeout()
	defer cancel()

	serverVersion, err := conn.ClientSet.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %v", err)
	}

	nodes, err := km.ListNodes(conn.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %v", err)
	}

	namespaces, err := conn.ClientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %v", err)
	}

	pods, err := km.ListPods(conn.ID, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to count pods: %v", err)
	}

	return &model.KubernetesCluster{
		ID:             conn.ID,
		Name:           conn.Name,
		APIEndpoint:    conn.Config.Host,
		Version:        serverVersion.GitVersion,
		Status:         conn.Status,
		NodeCount:      len(nodes),
		NamespaceCount: len(namespaces.Items),
		PodCount:       len(pods),
		LastSyncAt:     conn.LastSync,
	}, nil
}

func (km *K8sMonitor) ListNodes(clusterID string) ([]model.K8sNode, error) {
	conn, ok := km.GetCluster(clusterID)
	if !ok {
		return nil, fmt.Errorf("cluster not found: %s", clusterID)
	}

	ctx, cancel := k8sTimeout()
	defer cancel()
	nodes, err := conn.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %v", err)
	}

	result := make([]model.K8sNode, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		nodeInfo := km.convertNodeToModel(&node)
		result = append(result, nodeInfo)
	}

	return result, nil
}

func (km *K8sMonitor) convertNodeToModel(node *corev1.Node) model.K8sNode {
	allocatableCPU := node.Status.Allocatable.Cpu().String()
	capacityCPU := node.Status.Capacity.Cpu().String()
	allocatableMemory := node.Status.Allocatable.Memory().String()
	capacityMemory := node.Status.Capacity.Memory().String()

	var usedCPUPercent float64
	if capacityCPU != "" {
		if capQuantity, err := resource.ParseQuantity(capacityCPU); err == nil {
			if allocQuantity, err := resource.ParseQuantity(allocatableCPU); err == nil {
				used := capQuantity.Value() - allocQuantity.Value()
				usedCPUPercent = float64(used) / float64(capQuantity.MilliValue()) * 100.0
			}
		}
	}

	var usedMemPercent float64
	if capacityMemory != "" {
		if capQuantity, err := resource.ParseQuantity(capacityMemory); err == nil {
			if allocQuantity, err := resource.ParseQuantity(allocatableMemory); err == nil {
				used := capQuantity.Value() - allocQuantity.Value()
				usedMemPercent = float64(used) / float64(capQuantity.Value()) * 100.0
			}
		}
	}

	conditions := make([]model.NodeCondition, 0, len(node.Status.Conditions))
	for _, cond := range node.Status.Conditions {
		conditions = append(conditions, model.NodeCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}

	podsRunning := int(node.Status.Capacity.Pods().Value() - node.Status.Allocatable.Pods().Value())

	return model.K8sNode{
		Name:              node.Name,
		Ready:             isNodeReady(node),
		Architecture:      node.Status.NodeInfo.Architecture,
		OSImage:           node.Status.NodeInfo.OSImage,
		KernelVersion:     node.Status.NodeInfo.KernelVersion,
		KubeletVersion:    node.Status.NodeInfo.KubeletVersion,
		ContainerRuntime:  node.Status.NodeInfo.ContainerRuntimeVersion,
		CPU: model.K8sResourceInfo{
			Capacity:    capacityCPU,
			Allocatable: allocatableCPU,
			UsedPercent: usedCPUPercent,
		},
		Memory: model.K8sResourceInfo{
			Capacity:    capacityMemory,
			Allocatable: allocatableMemory,
			UsedPercent: usedMemPercent,
		},
		PodsAllocatable: int(node.Status.Allocatable.Pods().Value()),
		PodsCurrent:     podsRunning,
		Conditions:      conditions,
		Labels:          node.Labels,
		CreatedAt:       node.CreationTimestamp.Time,
	}
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (km *K8sMonitor) ListPods(clusterID, namespace, labelSelector string) ([]model.K8sPod, error) {
	conn, ok := km.GetCluster(clusterID)
	if !ok {
		return nil, fmt.Errorf("cluster not found: %s", clusterID)
	}

	ctx, cancel := k8sTimeout()
	defer cancel()

	listOpts := metav1.ListOptions{}
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	if labelSelector != "" {
		listOpts.LabelSelector = labelSelector
	}

	pods, err := conn.ClientSet.CoreV1().Pods(namespace).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	result := make([]model.K8sPod, 0, len(pods.Items))
	for _, pod := range pods.Items {
		podInfo := km.convertPodToModel(&pod, conn)
		result = append(result, podInfo)
	}

	return result, nil
}

func (km *K8sMonitor) convertPodToModel(pod *corev1.Pod, conn *K8sClusterConnection) model.K8sPod {
	readyContainers := int32(0)
	totalContainers := int32(len(pod.Spec.Containers))
	restartCount := int32(0)

	containers := make([]model.K8sContainer, 0, totalContainers)
	for _, c := range pod.Spec.Containers {
		containerStatus := getContainerStatus(&pod.Status, c.Name)
		if containerStatus != nil && containerStatus.Ready {
			readyContainers++
		}
		if containerStatus != nil {
			restartCount += containerStatus.RestartCount
		}

		var restartCountVal int32
		if containerStatus != nil {
			restartCountVal = containerStatus.RestartCount
		}

		container := model.K8sContainer{
			Name:         c.Name,
			Image:        c.Image,
			ImageID:      "",
			Ready:        containerStatus != nil && containerStatus.Ready,
			RestartCount: restartCountVal,
			State:        getContainerState(containerStatus),
			ResourceRequest: model.ResourceSpec{
				CPU:    c.Resources.Requests.Cpu().String(),
				Memory: c.Resources.Requests.Memory().String(),
			},
			ResourceLimit: model.ResourceSpec{
				CPU:    c.Resources.Limits.Cpu().String(),
				Memory: c.Resources.Limits.Memory().String(),
			},
			VolumeMounts: convertVolumeMounts(c.VolumeMounts),
			Ports:        convertContainerPorts(c.Ports),
		}

		containers = append(containers, container)
	}

	resourceRequests := calculatePodResources(pod.Spec.Containers, true)
	resourceLimits := calculatePodResources(pod.Spec.Containers, false)

	conditions := make([]model.PodCondition, 0, len(pod.Status.Conditions))
	for _, cond := range pod.Status.Conditions {
		conditions = append(conditions, model.PodCondition{
			Type:               string(cond.Type),
			Status:             string(cond.Status),
			LastTransitionTime: cond.LastTransitionTime.Time,
			Reason:             cond.Reason,
			Message:            cond.Message,
		})
	}

	events, _ := km.getPodEvents(conn, pod.Namespace, pod.Name)

	ownerRef := ""
	if len(pod.OwnerReferences) > 0 {
		ownerRef = pod.OwnerReferences[0].Name
	}

	qosClass := getQOSClass(pod)

	return model.K8sPod{
		Name:            pod.Name,
		Namespace:       pod.Namespace,
		Status:          string(pod.Status.Phase),
		Phase:           string(pod.Status.Phase),
		NodeName:        pod.Spec.NodeName,
		IP:              pod.Status.PodIP,
		StartTime:       pod.Status.StartTime.Time,
		RestartCount:    restartCount,
		ReadyContainers: readyContainers,
		TotalContainers: totalContainers,
		Labels:          pod.Labels,
		Annotations:     pod.Annotations,
		OwnerReference:  ownerRef,
		Containers:      containers,
		ResourceRequests: resourceRequests,
		ResourceLimits:  resourceLimits,
		Conditions:      conditions,
		Events:          events,
		QOSClass:        qosClass,
	}
}

func getContainerStatus(status *corev1.PodStatus, containerName string) *corev1.ContainerStatus {
	for _, cs := range status.ContainerStatuses {
		if cs.Name == containerName {
			return &cs
		}
	}
	return nil
}

func getContainerState(status *corev1.ContainerStatus) string {
	if status == nil {
		return "Unknown"
	}
	
	switch {
	case status.State.Running != nil:
		return "Running"
	case status.State.Waiting != nil:
		return status.State.Waiting.Reason
	case status.State.Terminated != nil:
		return "Terminated"
	default:
		return "Unknown"
	}
}

func convertVolumeMounts(mounts []corev1.VolumeMount) []model.VolumeMount {
	result := make([]model.VolumeMount, 0, len(mounts))
	for _, m := range mounts {
		result = append(result, model.VolumeMount{
			Name:      m.Name,
			MountPath: m.MountPath,
			SubPath:   m.SubPath,
			ReadOnly:  m.ReadOnly,
		})
	}
	return result
}

func convertContainerPorts(ports []corev1.ContainerPort) []model.ContainerPort {
	result := make([]model.ContainerPort, 0, len(ports))
	for _, p := range ports {
		result = append(result, model.ContainerPort{
			Name:          p.Name,
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      string(p.Protocol),
		})
	}
	return result
}

func calculatePodResources(containers []corev1.Container, request bool) model.K8sPodResources {
	var totalCPU, totalMemory resource.Quantity

	for _, c := range containers {
		var res corev1.ResourceList
		if request {
			res = c.Resources.Requests
		} else {
			res = c.Resources.Limits
		}

		if cpu, ok := res[corev1.ResourceCPU]; ok {
			totalCPU.Add(cpu)
		}
		if mem, ok := res[corev1.ResourceMemory]; ok {
			totalMemory.Add(mem)
		}
	}

	return model.K8sPodResources{
		CPU:    totalCPU.String(),
		Memory: totalMemory.String(),
	}
}

func getQOSClass(pod *corev1.Pod) string {
	requests := corev1.ResourceList{}
	limits := corev1.ResourceList{}
	for _, c := range pod.Spec.Containers {
		for name, qty := range c.Resources.Requests {
			if existing, ok := requests[name]; ok {
				qty.Add(existing)
			}
			requests[name] = qty
		}
		for name, qty := range c.Resources.Limits {
			if existing, ok := limits[name]; ok {
				qty.Add(existing)
			}
			limits[name] = qty
		}
	}

	isGuaranteed := func() bool {
		if len(requests) == 0 || len(limits) == 0 {
			return false
		}
		for name, req := range requests {
			if lim, exists := limits[name]; !exists || !lim.Equal(req) {
				return false
			}
		}
		return true
	}()

	isBestEffort := len(requests) == 0 && len(limits) == 0

	switch {
	case isGuaranteed:
		return "Guaranteed"
	case isBestEffort:
		return "BestEffort"
	default:
		return "Burstable"
	}
}

func (km *K8sMonitor) getPodEvents(conn *K8sClusterConnection, namespace, podName string) ([]model.K8sEvent, error) {
	ctx, cancel := k8sTimeout()
	defer cancel()
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName)

	events, err := conn.ClientSet.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	result := make([]model.K8sEvent, 0, len(events.Items))
	for _, event := range events.Items {
		result = append(result, model.K8sEvent{
			Type:      event.Type,
			Reason:    event.Reason,
			Message:   event.Message,
			Count:     event.Count,
			FirstSeen: event.FirstTimestamp.Time,
			LastSeen:  event.LastTimestamp.Time,
		})
	}

	return result, nil
}

func (km *K8sMonitor) ListNamespaces(clusterID string) ([]model.K8sNamespace, error) {
	conn, ok := km.GetCluster(clusterID)
	if !ok {
		return nil, fmt.Errorf("cluster not found: %s", clusterID)
	}

	ctx, cancel := k8sTimeout()
	defer cancel()
	namespaces, err := conn.ClientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %v", err)
	}

	result := make([]model.K8sNamespace, 0, len(namespaces.Items))
	for _, ns := range namespaces.Items {
		result = append(result, model.K8sNamespace{
			Name:      ns.Name,
			Status:    string(ns.Status.Phase),
			CreatedAt: ns.CreationTimestamp.Time,
			Labels:    ns.Labels,
		})
	}

	return result, nil
}

func (km *K8sMonitor) ListDeployments(clusterID, namespace string) ([]model.K8sDeployment, error) {
	conn, ok := km.GetCluster(clusterID)
	if !ok {
		return nil, fmt.Errorf("cluster not found: %s", clusterID)
	}

	ctx, cancel := k8sTimeout()
	defer cancel()
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	deployments, err := conn.ClientSet.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %v", err)
	}

	result := make([]model.K8sDeployment, 0, len(deployments.Items))
	for _, dep := range deployments.Items {
		conditions := make([]model.DeploymentCondition, 0, len(dep.Status.Conditions))
		for _, cond := range dep.Status.Conditions {
			conditions = append(conditions, model.DeploymentCondition{
				Type:           string(cond.Type),
				Status:         string(cond.Status),
				Reason:         cond.Reason,
				Message:        cond.Message,
				LastUpdateTime: cond.LastUpdateTime.Time,
			})
		}

		result = append(result, model.K8sDeployment{
			Name:               dep.Name,
			Namespace:          dep.Namespace,
			Replicas:           *dep.Spec.Replicas,
			ReadyReplicas:      dep.Status.ReadyReplicas,
			UpdatedReplicas:    dep.Status.UpdatedReplicas,
			AvailableReplicas:  dep.Status.AvailableReplicas,
			UnavailableReplicas: dep.Status.UnavailableReplicas,
			Strategy:          string(dep.Spec.Strategy.Type),
			Selector:          dep.Spec.Selector.MatchLabels,
			Conditions:        conditions,
			CreatedAt:         dep.CreationTimestamp.Time,
			Labels:            dep.Labels,
		})
	}

	return result, nil
}

func (km *K8sMonitor) ListServices(clusterID, namespace string) ([]model.K8sService, error) {
	conn, ok := km.GetCluster(clusterID)
	if !ok {
		return nil, fmt.Errorf("cluster not found: %s", clusterID)
	}

	ctx, cancel := k8sTimeout()
	defer cancel()
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	services, err := conn.ClientSet.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %v", err)
	}

	result := make([]model.K8sService, 0, len(services.Items))
	for _, svc := range services.Items {
		ports := make([]model.ServicePort, 0, len(svc.Spec.Ports))
		for _, p := range svc.Spec.Ports {
			sp := model.ServicePort{
				Name:       p.Name,
				Port:       p.Port,
				TargetPort: p.TargetPort.String(),
				Protocol:   string(p.Protocol),
			}
			if p.NodePort != 0 {
				sp.NodePort = p.NodePort
			}
			ports = append(ports, sp)
		}

		externalIPs := make([]string, 0)
		for _, ip := range svc.Status.LoadBalancer.Ingress {
			externalIPs = append(externalIPs, ip.IP)
		}

		result = append(result, model.K8sService{
			Name:            svc.Name,
			Namespace:       svc.Namespace,
			Type:            string(svc.Spec.Type),
			ClusterIP:       svc.Spec.ClusterIP,
			ExternalIPs:     externalIPs,
			Ports:           ports,
			Selector:        svc.Spec.Selector,
			SessionAffinity: string(svc.Spec.SessionAffinity),
			CreatedAt:       svc.CreationTimestamp.Time,
			Labels:          svc.Labels,
		})
	}

	return result, nil
}

func (km *K8sMonitor) RemoveCluster(clusterID string) error {
	km.clusterMutex.Lock()
	defer km.clusterMutex.Unlock()

	if _, ok := km.clusters[clusterID]; !ok {
		return fmt.Errorf("cluster not found: %s", clusterID)
	}

	delete(km.clusters, clusterID)
	logger.InfoLogger(fmt.Sprintf("Removed Kubernetes cluster: %s", clusterID))

	return nil
}

func (km *K8sMonitor) GetPodMetrics(clusterID, namespace, podName string) (*model.ContainerMetrics, error) {
	conn, ok := km.GetCluster(clusterID)
	if !ok {
		return nil, fmt.Errorf("cluster not found: %s", clusterID)
	}

	if conn.MetricsClient == nil {
		return nil, fmt.Errorf("metrics server not available")
	}

	ctx, cancel := k8sTimeout()
	defer cancel()
	podMetrics, err := conn.MetricsClient.MetricsV1beta1().PodMetricses(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod metrics: %v", err)
	}

	var totalCPUUsage, totalMemoryUsage uint64
	for _, container := range podMetrics.Containers {
		cpuQty := container.Usage[corev1.ResourceCPU]
		memQty := container.Usage[corev1.ResourceMemory]

		totalCPUUsage += uint64(cpuQty.MilliValue())
		totalMemoryUsage += uint64(memQty.Value())
	}

	return &model.ContainerMetrics{
		ContainerID:   podName,
		ContainerName: podName,
		Timestamp:     time.Now(),
		CPU: model.ContainerCPUMetrics{
			UsageNano: totalCPUUsage,
		},
		Memory: model.ContainerMemoryMetrics{
			Usage: totalMemoryUsage,
		},
	}, nil
}
