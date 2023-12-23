package controller

import (
	"context"
	"github.com/go-logr/logr"
	clusterconfigv1alpha1 "github.com/myoperator/clusterconfigoperator/pkg/apis/clusterconfig/v1alpha1"
	"github.com/myoperator/clusterconfigoperator/pkg/common"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type ClusterConfigController struct {
	client client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

func NewClusterConfigController(client client.Client, log logr.Logger, scheme *runtime.Scheme) *ClusterConfigController {
	return &ClusterConfigController{
		client: client,
		log:    log,
		Scheme: scheme,
	}
}

// Reconcile 调协 loop
func (r *ClusterConfigController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	// 调协时先获取该资源对象
	clusterconfig := &clusterconfigv1alpha1.ClusterConfig{}
	err := r.client.Get(ctx, req.NamespacedName, clusterconfig)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			klog.Error("get clusterconfig error: ", err)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
		}
		// 如果未找到的错误，不再进入调协
		return reconcile.Result{}, nil
	}

	// 处理删除状态，会等到 Finalizer 字段清空后才会真正删除
	// 1、删除所有 ns 下资源
	// 2、清空 Finalizer，更新状态
	if !clusterconfig.DeletionTimestamp.IsZero() {
		err = r.deleteResource(clusterconfig)
		if err != nil {
			klog.Error(err, "delete resource: ", clusterconfig.GetName()+"/"+clusterconfig.GetNamespace(), " failed")
			//mc.EventRecorder.Event(rr, corev1.EventTypeWarning, "Delete", fmt.Sprintf("delete %s fail", rr.Name))
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
		}
		klog.Info("successful delete clusterconfig")
		return reconcile.Result{}, nil
	}

	// 1. 先分割出目标 namespace
	namespaceList := splitString(clusterconfig.Spec.NamespaceList, ",")

	// 设置 crd 对象的 Finalizer 字段，并判断是否改变
	// 3. 检查是否已添加 Finalizer
	needToAdd := containsFinalizer(clusterconfig, namespaceList)
	if len(needToAdd) != 0 {
		// 添加 Finalizer
		for _, v := range needToAdd {
			controllerutil.AddFinalizer(clusterconfig, v)
			err = r.client.Update(ctx, clusterconfig)
			if err != nil {
				klog.Error("update clusterconfig finalizer err: ", err)
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
			}
		}
		// 需要注意
	}

	// 区分 configmaps or secrets
	switch clusterconfig.Spec.ConfigType {
	case common.ConfigMaps:
		// 处理 secrets 类型
		err = r.handleConfigmaps(clusterconfig)
		if err != nil {
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
		}
		// 处理 configmaps 类型
	case common.Secrets:
		// 处理 secrets 类型
		err = r.handleSecrets(clusterconfig)
		if err != nil {
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
		}
	}

	klog.Info("successful reconcile")

	return reconcile.Result{}, nil
}
