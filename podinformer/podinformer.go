// Package podinformer implements a pod discovery helper.
package podinformer

import (
	"context"
	"time"

	"log"

	"github.com/udhos/debounce/debounce"
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

	// ResyncPeriod is the resync period for the shared index informer.
	// If unset, the default is 0, meaning no resync.
	ResyncPeriod time.Duration

	// DebounceDelay is the delay between updates.
	// If unset, the default is 2 seconds.
	DebounceDelay time.Duration
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
	debouncer *debounce.Debouncer
}

// New creates an informer.
func New(options Options) *PodInformer {

	if options.OnUpdate == nil {
		panic("Options.OnUpdate is nil")
	}

	if options.Logf == nil {
		options.Logf = log.Printf
	}

	if options.DebounceDelay == 0 {
		options.DebounceDelay = 2 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	i := &PodInformer{
		options:   options,
		stopCh:    make(chan struct{}),
		cancelCtx: ctx,
		cancel:    cancel,
		debouncer: debounce.New(options.DebounceDelay),
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
		AddFunc: func(obj any) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			i.debugf("%s: add: '%s': error:%v", me, key, err)
			i.update()
		},
		UpdateFunc: func(obj, _ any) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			i.debugf("%s: update: '%s': error:%v", me, key, err)
			i.update()
		},
		DeleteFunc: func(obj any) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			i.debugf("%s: delete: '%s': error:%v", me, key, err)
			i.update()
		},
	})

	i.informer.Run(i.stopCh)

	return nil
}

// update calls the callback with the current list of pods.
// It uses a debouncer to coalesce updates.
func (i *PodInformer) update() {
	// Use debouncer to coalesce updates.
	// The debouncer delay ensures that we don't call the callback too often.
	i.debouncer.Run(i.listToCallback)
}

// listToCallback lists the pods and finally calls the OnUpdate callback.
func (i *PodInformer) listToCallback() {

	const me = "PodInformer.listToCallback"

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
	i.debouncer.Stop()
	i.cancel()
	close(i.stopCh)
}
