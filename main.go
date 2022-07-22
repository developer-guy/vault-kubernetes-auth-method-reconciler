package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/tools/cache"
)

func mainold() {
	nr := &NodeReconciler{}
	logf.SetLogger(zap.New())

	var log = logf.Log.WithName("builder-examples")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}
	err = nr.SetupWithManager(mgr)
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}

// NodeReconciler is a simple ControllerManagedBy example implementation.
type NodeReconciler struct {
	client.Client
}

// Reconcile is the implementation of the Reconcile interface.
// select master nodes only and display the status and the ip address of them
func (n *NodeReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	node := &corev1.Node{}
	err := n.Get(ctx, req.NamespacedName, node)
	if err != nil {
		return reconcile.Result{}, err
	}

	fmt.Printf("Node %s, Healthy: %t, IP: %s\n", node.Name, isNodeReady(node), node.Status.Addresses[0].Address)

	return reconcile.Result{}, nil
}

func isNodeReady(node *corev1.Node) bool {
	ready := true
	for _, condition := range node.Status.Conditions {
		switch condition.Type {
		case corev1.NodeReady:
			if condition.Status != corev1.ConditionTrue {
				ready = false
				break
			}
		case corev1.NodeMemoryPressure:
			if condition.Status != corev1.ConditionFalse {
				ready = false
				break
			}
		case corev1.NodeDiskPressure:
			if condition.Status != corev1.ConditionFalse {
				ready = false
				break
			}
		case corev1.NodeNetworkUnavailable:
			if condition.Status != corev1.ConditionFalse {
				ready = false
				break
			}
		}
	}
	return ready
}

func (n *NodeReconciler) InjectClient(c client.Client) error {
	n.Client = c
	return nil
}

func (n *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return builder.
		ControllerManagedBy(mgr). // Create the ControllerManagedBy
		For(&corev1.Node{}).
		WithEventFilter(selectOnlyMasterNode()).
		Complete(&NodeReconciler{}) // NodeReconciler is the Application API
}

func selectOnlyMasterNode() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		n := object.(*corev1.Node)
		_, master := n.Labels["node-role.kubernetes.io/master"]
		return master
	})
}

// informer way
func main() {
	c := config.GetConfigOrDie()

	clientSet, _ := kubernetes.NewForConfig(c)

	ctx := context.Background()
	sig := make(chan struct{})
	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)

	nodeNotReadyInformer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
				return clientSet.CoreV1().Nodes().List(ctx, options)
			},
			WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
				return clientSet.CoreV1().Nodes().Watch(ctx, options)
			},
		},
		&corev1.Node{},
		0,
	)

	// wq := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	// defer wq.Shutdown()

	nodeNotReadyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
		},
		UpdateFunc: func(oldObj, newObj interface{}) {

			switch newObj.(type) {
			case *corev1.Node:
				oldNode := oldObj.(*corev1.Node)
				newNode := newObj.(*corev1.Node)
				fmt.Printf("Old: %+v, New: %+v", oldNode.Labels, newNode.Labels)
			}

		},
		DeleteFunc: func(o interface{}) {
		},
	})

	println("start...")
	nodeNotReadyInformer.Run(sig)

	<-sigTerm
}
