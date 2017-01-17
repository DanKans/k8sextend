package k8sextend

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/resource"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Todo:
// - Is it possible to do something better with clientset type

// Namespace - contains information about pods
type Namespace struct {
	Name string
	Pods []Pod
}

// Pod -
type Pod struct {
	Name       string
	NodeName   string
	Containers []Container
	NodePTR    ClusterNode
}

// Container -
type Container struct {
	Container v1.Container
	// Limits describes the maximum amount of compute resources allowed.
	Limits v1.ResourceList
	// Requests describes the minimum amount of compute resources required.
	Requests v1.ResourceList
}

// ClusterNode -
type ClusterNode struct {
	Name string `json:"name"`
	// Allocatable represents the resources of a node that are available for scheduling.
	Allocatable v1.ResourceList `json:"allocatable"`
	// Capacity represents the total resources of a node.
	Capacity v1.ResourceList `json:"capacity"`
	// Available resources - introduced only here
	Available Resource `json:"available"`
	// Information about labels
	Labels map[string]string `json:"labels"`
}

// Resource create for internally use
type Resource struct {
	CPU *resource.Quantity `json:"CPU"`
	MEM *resource.Quantity `json:"MEM"`
	POD *resource.Quantity `json:"POD"`
}

// MyClientset -
type MyClientset struct {
	// MyClientset -
	*kubernetes.Clientset
}

// Global client
var client MyClientset

// Connect - Connect to kubernetes API
func Connect(Path string) (*[]Namespace, *[]ClusterNode) {

	var (
		clientset  *kubernetes.Clientset
		nss        []Namespace
		nodes      []ClusterNode
		err        error
		cfg        rest.Config
		tokenFile  string
		tokenBytes []byte
	)

	if Path == "" {

		tokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
		cfg.Host = "https://kubernetes.default.svc.cluster.local"
		cfg.CAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

		if _, err = os.Stat(cfg.CAFile); err != nil {
			panic(fmt.Errorf("File doesn't exist '%v'", err))
		}

		if _, err = os.Stat(tokenFile); err != nil {
			panic(fmt.Errorf("File doesn't exist '%v'", err))
		} else {
			tokenBytes, err = ioutil.ReadFile(tokenFile)
			cfg.BearerToken = string(tokenBytes)
			clientset = kubernetes.NewForConfigOrDie(&cfg)
		}

	} else {

		localFlag := flag.NewFlagSet("Local", flag.PanicOnError)

		kubeconfig := localFlag.String("kubeconfig", Path, "absolute path to the kubeconfig file")

		// uses the current context in kubeconfig
		config, _ := clientcmd.BuildConfigFromFlags("", *kubeconfig)

		// creates the clientset
		clientset, err = kubernetes.NewForConfig(config)

		if err != nil {
			panic(fmt.Errorf("Cannot connect to kubernetes '%v'", err))
		}

	}

	// creates local clientset type
	client = MyClientset{clientset}

	nss = client.GetNamespaces()
	nodes = client.GetNodes()

	// Loop through Namespaces (Pointer assignment)
	for _, nsValue := range nss {
		// Loop through Pods
		for podKey, podValue := range nsValue.Pods {
			// Loop through Nodes and compare names to assign proper Pointer
			for _, nodeValue := range nodes {
				if nodeValue.Name == podValue.NodeName {
					nsValue.Pods[podKey].NodePTR = nodeValue
				}
			}
		}
	}

	// Loop through Namespaces (Calculate Avalibility)
	for _, nsValue := range nss {
		// Loop through Pods
		for _, podValue := range nsValue.Pods {
			// Substract 1 from Available Pods
			podValue.NodePTR.Available.POD.Set(podValue.NodePTR.Available.POD.Value() - 1)
			// Loop through containers to substract Limits
			for _, cntValue := range podValue.Containers {
				if !(cntValue.Limits.Cpu().Value() == 0 || cntValue.Limits.Memory().Value() == 0) {
					podValue.NodePTR.Available.CPU.Sub(*cntValue.Limits.Cpu())
					podValue.NodePTR.Available.MEM.Sub(*cntValue.Limits.Memory())
				}
			}
		}
	}

	return &nss, &nodes
}

// Print - display info about ClusterNode
func (node *ClusterNode) Print() {
	//		(node.Available.MEM.Value() / (1024 * 1024 * 1024)), (node.Allocatable.Memory().Value() / (1024 * 1024 * 1024)),
	fmt.Printf("(%s) Available/Total: \tCPU:%v/%v \tMEM: %v/%v\t POD: %s/%s\n",
		node.Name,
		node.Available.CPU.MilliValue(), node.Allocatable.Cpu().MilliValue(),
		(node.Available.MEM.Value() / (1024 * 1024)), (node.Allocatable.Memory().Value() / (1024 * 1024)),
		node.Available.POD, node.Allocatable.Pods())
}

// GetNamespaces -
func (client *MyClientset) GetNamespaces() (nspace []Namespace) {

	reqNs, err := client.Core().Namespaces().List(v1.ListOptions{})

	if err != nil {
		panic(fmt.Errorf("Cannot GetNamespaces() '%v'", err))
	}

	for _, ns := range reqNs.Items {
		nspace = append(nspace,
			Namespace{ns.Name, client.GetPods(ns.Name)})
	}
	return
}

// GetPods -
func (client *MyClientset) GetPods(Namespace string) (pods []Pod) {

	var tmpPod Pod

	reqPods, err := client.Core().Pods(Namespace).List(v1.ListOptions{})

	if err != nil {
		panic(fmt.Errorf("Cannot GetPods() '%v'", err))
	}

	for _, pod := range reqPods.Items {
		tmpPod.Name = pod.Name
		tmpPod.NodeName = pod.Spec.NodeName

		for i := 0; i < len(pod.Spec.Containers); i++ {
			tmpPod.Containers = append(tmpPod.Containers, Container{pod.Spec.Containers[i],
				pod.Spec.Containers[i].Resources.Limits,
				pod.Spec.Containers[i].Resources.Requests})
		}

		pods = append(pods, tmpPod)
	}

	return
}

// GetNodes - Function used to get all nodes in cluster
func (client *MyClientset) GetNodes() (nodes []ClusterNode) {
	reqNodes, err := client.Core().Nodes().List(v1.ListOptions{})

	if err != nil {
		panic(fmt.Errorf("Cannot GetNodes() '%v'", err))
	}

	for _, node := range reqNodes.Items {

		resAvailable := Resource{node.Status.Allocatable.Cpu().Copy(), node.Status.Allocatable.Memory().Copy(), node.Status.Allocatable.Pods().Copy()}

		nodes = append(nodes,
			ClusterNode{node.Name,
				node.Status.Allocatable,
				node.Status.Capacity,
				resAvailable,
				node.Labels})
	}

	return
}
