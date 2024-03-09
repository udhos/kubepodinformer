// Package main implements the example.
package main

import (
	"log"
	"os"
	"runtime"
	"time"

	"github.com/udhos/kube/kubeclient"
	"github.com/udhos/kubepodinformer/podinformer"
)

func main() {

	//
	// interval
	//
	intervalStr := os.Getenv("INTERVAL")
	interval, errConv := time.ParseDuration(intervalStr)
	if errConv != nil {
		log.Printf("INTERVAL='%s': %v", intervalStr, errConv)
		interval = 10 * time.Minute
	}
	log.Printf("INTERVAL='%s' interval=%v", intervalStr, interval)

	//
	// label selector
	//
	labelSelector := "app=miniapi"
	ls := os.Getenv("LABEL_SELECTOR")
	if ls != "" {
		labelSelector = ls
	}
	log.Printf("LABEL_SELECTOR='%s' label_selector=%s", ls, labelSelector)

	//
	// resync period
	//
	resyncStr := os.Getenv("RESYNC_PERIOD")
	resync, errSync := time.ParseDuration(resyncStr)
	if errSync != nil {
		log.Printf("RESYNC_PERIOD='%s': %v", resyncStr, errSync)
	}
	log.Printf("RESYNC_PERIOD='%s' resync_period=%v", resyncStr, resync)

	//
	// kube client
	//
	clientOptions := kubeclient.Options{DebugLog: true}
	clientset, errClientset := kubeclient.New(clientOptions)
	if errClientset != nil {
		log.Fatalf("kube clientset error: %v", errClientset)
	}

	options := podinformer.Options{
		Client:        clientset,
		Namespace:     "default",
		LabelSelector: labelSelector,
		OnUpdate:      onUpdate,
		DebugLog:      false,
		ResyncPeriod:  resync,
	}

	const limit = 50000

	for {
		for i := 0; i < limit; i++ {
			once(options)
		}
		log.Printf("executed: %d", limit)
		time.Sleep(time.Second)
	}
}

func once(options podinformer.Options) {
	informer := podinformer.New(options)

	go func() {
		errRun := informer.Run()
		if errRun != nil {
			log.Printf("informer run error: %v", errRun)
		}
	}()

	runtime.Gosched()

	informer.Stop()
}

func onUpdate(pods []podinformer.Pod) {
	const me = "onUpdate"
	log.Printf("%s: %d", me, len(pods))
	for i, p := range pods {
		log.Printf("%s: %d/%d: namespace=%s pod=%s ip=%s ready=%t",
			me, i, len(pods), p.Namespace, p.Name, p.IP, p.Ready)
	}
}
