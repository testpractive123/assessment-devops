package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("error getting user home dir: %v\n", err)
		os.Exit(1)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	fmt.Printf("Using kubeconfig: %s\n", kubeConfigPath)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		fmt.Printf("Error getting Kubernetes config: %v\n", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		fmt.Printf("Error creating Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	// List all namespaces
	namespaces, err := ListNamespaces(clientset)
	if err != nil {
		fmt.Printf("Error listing namespaces: %v\n", err)
		os.Exit(1)
	}

	for _, namespace := range namespaces.Items {
		fmt.Printf("Processing namespace: %s\n", namespace.Name)
		pods, err := ListPods(namespace.Name, clientset)
		if err != nil {
			fmt.Printf("Error listing pods in namespace %s: %v\n", namespace.Name, err)
			continue
		}

		for _, pod := range pods.Items {
			if strings.Contains(pod.Name, "database") {
				fmt.Printf("Pod with 'database' found: %s\n", pod.Name)
				if err := RestartDeployment(pod.Namespace, pod.Name, clientset); err != nil {
					fmt.Printf("Error restarting deployment for pod %s: %v\n", pod.Name, err)
				}
			}
		}
	}
}

func ListPods(namespace string, client kubernetes.Interface) (*v1.PodList, error) {
	fmt.Printf("Listing pods in namespace %s\n", namespace)
	pods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting pods: %v", err)
	}
	return pods, nil
}

func ListNamespaces(client kubernetes.Interface) (*v1.NamespaceList, error) {
	fmt.Println("Listing namespaces")
	namespaces, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting namespaces: %v", err)
	}
	return namespaces, nil
}

func RestartDeployment(namespace string, podName string, client kubernetes.Interface) error {
	// Find the deployment associated with the pod
	deploymentClient := client.AppsV1().Deployments(namespace)
	deployments, err := deploymentClient.List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", strings.Split(podName, "-")[0]),
	})
	if err != nil {
		return fmt.Errorf("error listing deployments: %v", err)
	}

	if len(deployments.Items) == 0 {
		return fmt.Errorf("no deployments found for pod %s", podName)
	}

	// Assuming the pod name contains a unique identifier for the deployment
	deploymentName := strings.Split(podName, "-")[0]
	fmt.Printf("Restarting deployment: %s\n", deploymentName)

	deployment, err := deploymentClient.Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting deployment: %v", err)
	}

	// Trigger a rollout restart by updating an annotation
	deployment.Spec.Template.Annotations = map[string]string{
		"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
	}

	_, err = deploymentClient.Update(context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error updating deployment: %v", err)
	}

	return nil
}
