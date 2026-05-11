/*
Copyright 2024 liuchong.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
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
