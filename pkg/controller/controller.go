package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	clusterconfigv1alpha1 "github.com/myoperator/clusterconfigoperator/pkg/apis/clusterconfig/v1alpha1"
	"github.com/myoperator/clusterconfigoperator/pkg/common"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sort"
	"strings"
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
			return reconcile.Result{}, err
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
				return reconcile.Result{}, err
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
			return reconcile.Result{}, err
		}
		// 处理 configmaps 类型
	case common.Secrets:
		// 处理 secrets 类型
		err = r.handleSecrets(clusterconfig)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	klog.Info("successful reconcile")

	return reconcile.Result{}, nil
}

// deleteResource 清理资源对象逻辑
func (r *ClusterConfigController) deleteResource(clusterConfig *clusterconfigv1alpha1.ClusterConfig) error {
	// 1. 先分割出目标 namespace
	namespaceList := splitString(clusterConfig.Spec.NamespaceList, ",")

	// 2. 遍历 namespace
	// 先去各个 namespace 查找是否存在，
	// 如果不存在，则创建，
	// 如果已经存在，则比较 data 字段是否一致，如果不一致则修改
	// FIXME: 注意这里会有一种情况：就是修改 namespaceList 结果该删除的未删除的情况
	for _, namespace := range namespaceList {

		switch clusterConfig.Spec.ConfigType {
		case common.ConfigMaps:
			toConfigMap := &v1.ConfigMap{}
			err := r.client.Get(context.Background(), client.ObjectKey{Name: clusterConfig.Name, Namespace: namespace}, toConfigMap)
			if err != nil {
				if errors.IsNotFound(err) {

					klog.Info("[toConfigMap] notfound ")
					return nil
				}
				klog.Error(err, "[toConfigMap] Failed to get")
				return err
			}
			err = r.client.Delete(context.Background(), toConfigMap)
			if err != nil {
				klog.Error(err, "[toConfigMap] Failed to delete")
				return err
			}
		case common.Secrets:
			toSecret := &v1.Secret{}
			err := r.client.Get(context.Background(), client.ObjectKey{Name: clusterConfig.Name, Namespace: namespace}, toSecret)
			if err != nil {
				if errors.IsNotFound(err) {

					klog.Info("[toSecret] notfound ")
					return nil
				}
				klog.Error(err, "[toSecret] Failed to get")
				return err
			}
			err = r.client.Delete(context.Background(), toSecret)
			if err != nil {
				klog.Error(err, "[toSecret] Failed to delete")
				return err
			}
		}

		// 清理完成后，从 Finalizers 中移除 Finalizer
		controllerutil.RemoveFinalizer(clusterConfig, namespace)
		err := r.client.Update(context.Background(), clusterConfig)
		if err != nil {
			klog.Error("clean clusterConfig finalizer err: ", err)
			return err
		}
	}

	return nil
}

// handleConfigmaps 处理 configmaps 资源对象
func (r *ClusterConfigController) handleConfigmaps(clusterConfig *clusterconfigv1alpha1.ClusterConfig) error {

	// 1. 先分割出目标 namespace
	namespaceList := splitString(clusterConfig.Spec.NamespaceList, ",")

	// 2. 遍历 namespace
	// 先去各个 namespace 查找是否存在，
	// 如果不存在，则创建，
	// 如果已经存在，则比较 data 字段是否一致，如果不一致则修改
	// FIXME: 注意这里会有一种情况：就是修改 namespaceList 结果该删除的未删除的情况
	for _, namespace := range namespaceList {
		fmt.Println(namespace)
		toConfigMap := &v1.ConfigMap{}
		err := r.client.Get(context.Background(), client.ObjectKey{Name: clusterConfig.Name, Namespace: namespace}, toConfigMap)
		if err != nil {
			if errors.IsNotFound(err) {
				toConfigMap = newConfigMap(clusterConfig, namespace)

				err = r.client.Create(context.Background(), toConfigMap, &client.CreateOptions{})
				if err != nil {
					klog.Error(err, "[toConfigMap] Failed to create")
					return err
				}
				klog.Info("[toConfigMap] Created")
				return nil
			}
			klog.Error(err, "[toConfigMap] Failed to get")
			return err
		}

		// Update toSecret data if data is changed.
		if !reflect.DeepEqual(toConfigMap.Data, clusterConfig.Spec.Data) {
			toConfigMap.Data = clusterConfig.Spec.Data
			err = r.client.Update(context.Background(), toConfigMap, &client.UpdateOptions{})
			if err != nil {
				klog.Error(err, "[toConfigMap] Failed to update")
				return err
			}
			klog.Info("[toConfigMap] Updated with clusterConfig.Spec.Data")
		}

	}

	return nil
}

// handleSecrets 处理 secrets 资源对象
func (r *ClusterConfigController) handleSecrets(clusterConfig *clusterconfigv1alpha1.ClusterConfig) error {
	a := make(map[string][]byte, 0)

	for i, k := range clusterConfig.Spec.Data {
		a[i] = []byte(k)
	}

	// 1. 先分割出目标 namespace
	namespaceList := splitString(clusterConfig.Spec.NamespaceList, ",")

	// 2. 遍历 namespace
	// 先去各个 namespace 查找是否存在，
	// 如果不存在，则创建，
	// 如果已经存在，则比较 data 字段是否一致，如果不一致则修改
	for _, namespace := range namespaceList {
		fmt.Println(namespace)
		toSecret := &v1.Secret{}
		err := r.client.Get(context.Background(), client.ObjectKey{Name: clusterConfig.Name, Namespace: namespace}, toSecret)
		if err != nil {
			if errors.IsNotFound(err) {
				toSecret = newSecret(clusterConfig, namespace, a)

				// FIXME: cross-namespace owner references are disallowed, owner's namespace default, obj's namespace test[toSecret] Failed to set controller reference
				//err := controllerutil.SetControllerReference(clusterConfig, toSecret, r.Scheme)
				//if err != nil {
				//	klog.Error(err, "[toSecret] Failed to set controller reference")
				//	return err
				//}

				err = r.client.Create(context.Background(), toSecret, &client.CreateOptions{})
				if err != nil {
					klog.Error(err, "[toSecret] Failed to create")
					return err
				}
				klog.Info("[toSecret] Created")
				return nil
			}
			klog.Error(err, "[toSecret] Failed to get")
			return err
		}

		// FIXME 报错
		// 6. Check if `toSecret` is managed by secret-mirror-controller.
		//if !metav1.IsControlledBy(toSecret, clusterConfig) {
		//	klog.Error(err, "[toSecret] Not controlled by SecretMirror")
		//	return err
		//}

		// Update toSecret data if data is changed.
		if !reflect.DeepEqual(toSecret.Data, a) {
			toSecret.Data = a
			err = r.client.Update(context.Background(), toSecret, &client.UpdateOptions{})
			if err != nil {
				klog.Error(err, "[toSecret] Failed to update")
				return err
			}
			klog.Info("[toSecret] Updated with fromSecret.Data")
		}

	}

	return nil
}

func newSecret(clusterConfig *clusterconfigv1alpha1.ClusterConfig, namespace string, secretData map[string][]byte) *v1.Secret {
	toSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterConfig.Name,
			Namespace: namespace,
		},
		Data: secretData,
	}
	return toSecret
}

func newConfigMap(clusterConfig *clusterconfigv1alpha1.ClusterConfig, namespace string) *v1.ConfigMap {
	toSecret := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterConfig.Name,
			Namespace: namespace,
		},
		Data: clusterConfig.Spec.Data,
	}
	return toSecret
}

func splitString(input, separator string) []string {
	// 去除空格
	input = strings.ReplaceAll(input, " ", "")
	// 使用 strings.Split 进行分割
	result := strings.Split(input, separator)
	// 排序
	sort.StringSlice(result).Sort()
	return result
}

// 查找是否含有 Finalizer 字段
func containsFinalizer(clusterconfig *clusterconfigv1alpha1.ClusterConfig, namespaceList []string) []string {
	needToAddFinalizer := make([]string, 0)

	for _, namespace := range namespaceList {
		found := false

		for _, finalizer := range clusterconfig.Finalizers {
			if finalizer == namespace {
				found = true
				break
			}
		}

		if !found {
			needToAddFinalizer = append(needToAddFinalizer, namespace)
		}
	}

	return needToAddFinalizer
}
