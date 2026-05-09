/*
Copyright 2024 liuchong.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pkg

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func MinHpaReplicas(ctx context.Context, clientset *kubernetes.Clientset, namespace string, name string) (minreplicas int32, err error) {
	l := log.FromContext(ctx)
	hpaInfo, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			l.Error(err, "HPA not found", "namespace", namespace, "name", name)
			return -1, err
		}
		l.Error(err, "HPA error", "namespace", namespace, "name", name)
		return -1, err
	}
	minreplicas = *hpaInfo.Spec.MinReplicas
	return minreplicas, nil
}

func UpdateMinHpaReplicas(ctx context.Context, clientset *kubernetes.Clientset, namespace string, name string, targetRep int32) (err error) {
	l := log.FromContext(ctx)
	hpaInfo, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			l.Error(err, "HPA not found", "namespace", namespace, "name", name)
			return err
		}
		l.Error(err, "HPA error", "namespace", namespace, "name", name)
		return err
	}
	//计算差值
	//扩容情况
	result := targetRep - *hpaInfo.Spec.MinReplicas
	hpaInfo.Spec.MinReplicas = &targetRep
	hpaInfo.Spec.MaxReplicas += result
	newhpa, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Update(ctx, hpaInfo, metav1.UpdateOptions{})
	if err != nil {
		l.Error(err, "update hpa info failed", "namespace", namespace, "name", name)
		return err
	}
	l.Info("new hpa info", "namespace", namespace, "name", name, "minReplicas", *newhpa.Spec.MinReplicas, "maxReplicas", newhpa.Spec.MaxReplicas)
	return
}
