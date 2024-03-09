// Package main implements the example.
package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/udhos/kubepodinformer/podinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {

	intervalStr := os.Getenv("INTERVAL")
	interval, errConv := time.ParseDuration(intervalStr)
	if errConv != nil {
		log.Printf("INTERVAL='%s': %v", intervalStr, errConv)
		interval = 10 * time.Minute
	}
	log.Printf("INTERVAL='%s' interval=%v", intervalStr, interval)

	clientset, errClientset := newKubeClient()
	if errClientset != nil {
		log.Fatalf("kube clientset error: %v", errClientset)
	}

	options := podinformer.Options{
		Client:        clientset,
		Namespace:     "default",
		LabelSelector: "app=miniapi",
		OnUpdate:      onUpdate,
		DebugLog:      false,
	}

	informer := podinformer.New(options)

	go func() {
		log.Printf("######## main: time limit: %v - begin", interval)
		time.Sleep(interval)
		log.Printf("######## main: time limit: %v - end", interval)
		informer.Stop()
	}()

	errRun := informer.Run()
	log.Printf("informer run error: %v", errRun)
}

func onUpdate(pods []podinformer.Pod) {
	const me = "onUpdate"
	log.Printf("%s: %d", me, len(pods))
	for i, p := range pods {
		log.Printf("%s: %d/%d: namespace=%s pod=%s ip=%s ready=%t",
			me, i, len(pods), p.Namespace, p.Name, p.IP, p.Ready)
	}
}

func newKubeClient() (*kubernetes.Clientset, error) {
	const me = "newKubeClient"

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, errHome := os.UserHomeDir()
		if errHome != nil {
			log.Printf("%s: could not get home dir: %v", me, errHome)
		}
		kubeconfig = filepath.Join(home, "/.kube/config")
	}

	config, errKubeconfig := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if errKubeconfig != nil {
		log.Printf("%s: kubeconfig: %v", me, errKubeconfig)

		c, errInCluster := rest.InClusterConfig()
		if errInCluster != nil {
			log.Printf("%s: in-cluster-config: %v", me, errInCluster)
		}
		config = c
	}

	if config == nil {
		return nil, errors.New("could not get cluster config")
	}

	clientset, errConfig := kubernetes.NewForConfig(config)

	return clientset, errConfig
}
