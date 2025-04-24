// Package podinformer implements a pod discovery helper.
package podinformer

import (
	"context"
	"time"

	"log"

	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Options define config for informer.
type Options struct {
	// Client provides Clientset.
	Client *kubernetes.Clientset

	// Restrict namespace.
	Namespace string

	// LabelSelector restricts pods by label.
	// Empty LabelSelector matches everything.
	// Example: "app=miniapi,tier=backend"
	LabelSelector string

	// OnUpdate is required callback function for POD discovery.
	OnUpdate func(pods []Pod)

	// Logf provides logging fuction. If undefined, defaults to log.Printf.
	Logf func(format string, v ...any)

	// DebugLog enables debug logs.
	DebugLog bool

	ResyncPeriod time.Duration
}

// Pod holds information about discovered pod.
type Pod struct {
	Namespace string
	Name      string
	IP        string
	Ready     bool
}

// PodInformer holds informer state.
type PodInformer struct {
	options   Options
	stopCh    chan struct{}
	cancelCtx context.Context
	cancel    func()
	informer  cache.SharedIndexInformer
}

// New creates an informer.
func New(options Options) *PodInformer {

	if options.OnUpdate == nil {
		panic("Options.OnUpdate is nil")
	}

	if options.Logf == nil {
		options.Logf = log.Printf
	}

	ctx, cancel := context.WithCancel(context.Background())

	i := &PodInformer{
		options:   options,
		stopCh:    make(chan struct{}),
		cancelCtx: ctx,
		cancel:    cancel,
	}

	return i
}

func (i *PodInformer) debugf(format string, v ...any) {
	if i.options.DebugLog {
		i.options.Logf("DEBUG podinformer: "+format, v...)
	}
}

func (i *PodInformer) errorf(format string, v ...any) {
	i.options.Logf("ERROR podinformer: "+format, v...)
}

// Run runs the informer.
func (i *PodInformer) Run() error {

	const me = "PodInformer.Run"

	listWatch := &cache.ListWatch{
		ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
			options.LabelSelector = i.options.LabelSelector
			return i.options.Client.CoreV1().Pods(i.options.Namespace).List(i.cancelCtx, options)
		},
		WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = i.options.LabelSelector
			return i.options.Client.CoreV1().Pods(i.options.Namespace).Watch(i.cancelCtx, options)
		},
	}

	i.informer = cache.NewSharedIndexInformer(
		listWatch,
		&core_v1.Pod{},
		i.options.ResyncPeriod,
		cache.Indexers{},
	)

	i.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			i.debugf("%s: add: '%s': error:%v", me, key, err)
			i.update()
		},
		UpdateFunc: func(obj, _ interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			i.debugf("%s: update: '%s': error:%v", me, key, err)
			i.update()
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			i.debugf("%s: delete: '%s': error:%v", me, key, err)
			i.update()
		},
	})

	i.informer.Run(i.stopCh)

	return nil
}

func (i *PodInformer) update() {

	const me = "PodInformer.update"

	list := i.informer.GetStore().List()
	size := len(list)

	i.debugf("%s: listing pods: %d", me, size)

	pods := make([]Pod, 0, size)

	for _, obj := range list {
		pod, ok := obj.(*core_v1.Pod)
		if !ok {
			i.errorf("%s: unexpected object type: %T", me, obj)
			continue
		}
		p := Pod{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			IP:        pod.Status.PodIP,
			Ready:     isPodReady(pod),
		}
		pods = append(pods, p)
	}

	i.options.OnUpdate(pods)
}

func isPodReady(pod *core_v1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == core_v1.PodReady && condition.Status == core_v1.ConditionTrue {
			return true
		}
	}
	return false
}

// Stop stops the informer to release resources.
func (i *PodInformer) Stop() {
	i.cancel()
	close(i.stopCh)
}
