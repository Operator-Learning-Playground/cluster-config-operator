package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	clusterconfigv1alpha1 "github.com/myoperator/clusterconfigoperator/pkg/apis/clusterconfig/v1alpha1"
	"github.com/myoperator/clusterconfigoperator/pkg/common"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type ClusterConfigController struct {
	// 增加事件通知器
	client        client.Client
	Scheme        *runtime.Scheme
	log           logr.Logger
	EventRecorder record.EventRecorder
}

func NewClusterConfigController(client client.Client, log logr.Logger, scheme *runtime.Scheme, eventRecorder record.EventRecorder) *ClusterConfigController {
	return &ClusterConfigController{
		client:        client,
		log:           log,
		Scheme:        scheme,
		EventRecorder: eventRecorder,
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

	if clusterconfig.Status.ProcessedNamespace == nil {
		clusterconfig.Status.ProcessedNamespace = make([]string, 0)
	}

	// 处理删除状态，会等到 Finalizer 字段清空后才会真正删除
	// 1、删除所有 ns 下资源
	// 2、清空 Finalizer，更新状态
	if !clusterconfig.DeletionTimestamp.IsZero() {
		err = r.deleteResource(ctx, clusterconfig)
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

	/* FIXME: 如果要实现类似管理特定 namespace 功能，可能需要一个 status 记录 已经创建完成的 namespaceList
	1. 进入调协时，先比对 namespaceList 与 status namespaceList 的区别，如果
	status namespaceList 中有 namespaceList 没有的部分，直接删除
	2. 当创建时，就加入 status namespaceList list 中
	*/

	// 如果 cr 的 status ProcessedNamespace 字段长度不为 0，代表已经是处理后的资源对象，需要进入
	if len(clusterconfig.Status.ProcessedNamespace) != 0 {
		resList := calculateNeedToDeleteNamespace(namespaceList, clusterconfig.Status.ProcessedNamespace)
		// 遍历删除此namespace下的资源对象
		err := r.deleteResourceByNamespace(ctx, clusterconfig, resList)
		if err != nil {
			klog.Error(err, "delete resource: ", clusterconfig.GetName()+"/"+clusterconfig.GetNamespace(), " failed")
			r.EventRecorder.Eventf(clusterconfig, v1.EventTypeWarning, "Delete", fmt.Sprintf("delete %s clusterConfig error: %s", clusterconfig.Name, err.Error()))
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
		}
		// 更新 status 字段
		clusterconfig.Status.ProcessedNamespace = namespaceList
		err = r.client.Status().Update(ctx, clusterconfig)
		if err != nil {
			r.EventRecorder.Eventf(clusterconfig, v1.EventTypeWarning, "UpdateStatus", fmt.Sprintf("update %s clusterConfig status error: %s", clusterconfig.Name, err.Error()))
			klog.Error("update clusterconfig status err: ", err)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
		}
	}

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
				r.EventRecorder.Eventf(clusterconfig, v1.EventTypeWarning, "UpdateStatus", fmt.Sprintf("update %s clusterConfig finalizer error: %s", clusterconfig.Name, err.Error()))
				return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
			}
		}
		// 需要注意
	}

	// 区分 configmaps or secrets
	switch clusterconfig.Spec.ConfigType {
	case common.ConfigMaps:
		// 处理 secrets 类型
		err = r.handleConfigmaps(ctx, clusterconfig)
		if err != nil {
			r.EventRecorder.Eventf(clusterconfig, v1.EventTypeWarning, "Handle", fmt.Sprintf("handle %s clusterConfig configmap error: %s", clusterconfig.Name, err.Error()))
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
		}
		// 处理 configmaps 类型
	case common.Secrets:
		// 处理 secrets 类型
		err = r.handleSecrets(ctx, clusterconfig)
		if err != nil {
			r.EventRecorder.Eventf(clusterconfig, v1.EventTypeWarning, "Handle", fmt.Sprintf("handle %s clusterConfig secrets error: %s", clusterconfig.Name, err.Error()))
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
		}
	}

	// 更新 status 字段
	clusterconfig.Status.ProcessedNamespace = namespaceList
	err = r.client.Status().Update(ctx, clusterconfig)
	if err != nil {
		r.EventRecorder.Eventf(clusterconfig, v1.EventTypeWarning, "UpdateStatus", fmt.Sprintf("update %s clusterConfig status error: %s", clusterconfig.Name, err.Error()))
		klog.Error("update clusterconfig status err: ", err)
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 60}, err
	}

	klog.Info("successful reconcile")

	return reconcile.Result{}, nil
}

func (r *ClusterConfigController) OnUpdateConfigHandlerByClusterConfig(event event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
	for _, ref := range event.ObjectNew.GetOwnerReferences() {
		if ref.Kind == clusterconfigv1alpha1.ClusterConfigKind && ref.APIVersion == clusterconfigv1alpha1.ClusterConfigApiVersion {
			// 重新放入 Reconcile 调协方法
			limitingInterface.Add(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ref.Name, Namespace: event.ObjectNew.GetNamespace(),
				},
			})
		}
	}
}

func (r *ClusterConfigController) OnDeleteConfigHandlerByClusterConfig(event event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
	for _, ref := range event.Object.GetOwnerReferences() {
		if ref.Kind == clusterconfigv1alpha1.ClusterConfigKind && ref.APIVersion == clusterconfigv1alpha1.ClusterConfigApiVersion {
			// 重新入列
			klog.Info("delete pod: ", event.Object.GetName(), event.Object.GetObjectKind())
			limitingInterface.Add(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: ref.Name,
					Namespace: event.Object.GetNamespace()}})
		}
	}
}
