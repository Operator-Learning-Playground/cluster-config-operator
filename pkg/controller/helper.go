package controller

import (
	"context"
	clusterconfigv1alpha1 "github.com/myoperator/clusterconfigoperator/pkg/apis/clusterconfig/v1alpha1"
	"github.com/myoperator/clusterconfigoperator/pkg/common"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sort"
	"strings"
)

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
	klog.Infof("namespace list: %v\n", namespaceList)
	// 2. 遍历 namespace
	// 先去各个 namespace 查找是否存在，
	// 如果不存在，则创建，
	// 如果已经存在，则比较 data 字段是否一致，如果不一致则修改
	// FIXME: 注意这里会有一种情况：就是修改 namespaceList 结果该删除的未删除的情况
	for _, namespace := range namespaceList {
		klog.Infof("namespace to create configmaps: %v\n", namespace)
		toConfigMap := &v1.ConfigMap{}
		err := r.client.Get(context.Background(), client.ObjectKey{Name: clusterConfig.Name, Namespace: namespace}, toConfigMap)
		if err != nil {
			if errors.IsNotFound(err) {
				toConfigMap = newConfigMap(clusterConfig, namespace)

				err = r.client.Create(context.Background(), toConfigMap, &client.CreateOptions{})
				if err != nil {
					klog.Errorf("[toConfigMap] in [%v] namespace Failed to create error: %v\n", namespace, err)
					return err
				}
				klog.Infof("[toConfigMap] Created in [%v] namespace\n", namespace)
			} else {
				klog.Errorf("[toConfigMap] Failed to get in [%v] namespace, error: %v", namespace, err)
				return err
			}

		}

		// Update toSecret data if data is changed.
		if !reflect.DeepEqual(toConfigMap.Data, clusterConfig.Spec.Data) {
			toConfigMap.Data = clusterConfig.Spec.Data
			err = r.client.Update(context.Background(), toConfigMap, &client.UpdateOptions{})
			if err != nil {
				klog.Errorf("[toConfigMap] in [%v] namespace Failed to update error: %v\n", namespace, err)
				return err
			}
			klog.Infof("[toConfigMap] Updated with clusterConfig.Spec.Data in [%v] namespace\n", namespace)
		}

	}

	return nil
}

// handleSecrets 处理 secrets 资源对象
func (r *ClusterConfigController) handleSecrets(clusterConfig *clusterconfigv1alpha1.ClusterConfig) error {
	// 處理 string -> []byte
	a := make(map[string][]byte, 0)

	for i, k := range clusterConfig.Spec.Data {
		a[i] = []byte(k)
	}

	// 1. 先分割出目标 namespace
	namespaceList := splitString(clusterConfig.Spec.NamespaceList, ",")
	klog.Infof("namespace list: %v\n", namespaceList)
	// 2. 遍历 namespace
	// 先去各个 namespace 查找是否存在，
	// 如果不存在，则创建，
	// 如果已经存在，则比较 data 字段是否一致，如果不一致则修改
	for _, namespace := range namespaceList {
		klog.Infof("namespace to create secret: %v\n", namespace)
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
					klog.Errorf("[toSecret] in [%v] namespace Failed to create error: %v\n", namespace, err)
					return err
				}
				klog.Infof("[toSecret] Created in [%v] namespace\n", namespace)
			} else {
				klog.Errorf("[toSecret] Failed to get in [%v] namespace, error: %v", namespace, err)
				return err
			}
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
				klog.Errorf("[toSecret] in [%v] namespace Failed to update error: %v\n", namespace, err)
				return err
			}
			klog.Infof("[toSecret] Updated with clusterConfig.Spec.Data in [%v] namespace\n", namespace)

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

