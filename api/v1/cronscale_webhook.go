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

package v1

import (
	"errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var cronscalelog = logf.Log.WithName("cronscale-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *CronScale) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-application-liuchong-cn-v1-cronscale,mutating=true,failurePolicy=fail,sideEffects=None,groups=application.liuchong.cn,resources=cronscales,verbs=create;update,versions=v1,name=mcronscale.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &CronScale{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *CronScale) Default() {
	cronscalelog.Info("default", "name", r.Name)
	deployName := r.Spec.DeploymentName
	//当关联的deployment名称是空，默认给一个nginx
	if len(deployName) == 0 {
		deployName = "nginx"
	}
	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-application-liuchong-cn-v1-cronscale,mutating=false,failurePolicy=fail,sideEffects=None,groups=application.liuchong.cn,resources=cronscales,verbs=create;update,versions=v1,name=vcronscale.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &CronScale{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *CronScale) ValidateCreate() (admission.Warnings, error) {
	cronscalelog.Info("validate create", "name", r.Name)
	// TODO(user): fill in your validation logic upon object creation.
	return r.validateCronScale()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *CronScale) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	cronscalelog.Info("validate update", "name", r.Name)
	// TODO(user): fill in your validation logic upon object update.
	return r.validateCronScale()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *CronScale) ValidateDelete() (admission.Warnings, error) {
	cronscalelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

// 1.扩容的数量一定要大于缩容的数量
// 2.yaml文件的字段不能为空
// 3.HPA存在的条件下：扩容配置的副本数要大于HPA的最小副本数，缩容配置的副本数要小于HPA的最小副本数
func (r *CronScale) validateCronScale() (admission.Warnings, error) {
	cronscalelog.Info("validate CronScale", "name", r.Name)
	addR, minR := r.Spec.Add.TargetReplicas, r.Spec.Minus.TargetReplicas
	//扩容数量大于缩容的数量
	if addR <= minR {
		cronscalelog.Error(errors.New("spec.addTargetReplicas Err"), "扩容数量小于缩容数量")
		return nil, errors.New("spec.addTargetReplicas set Err")
	}
	//检查字段
	if len(r.Spec.Add.ScaleTime) == 0 || r.Spec.Add.TargetReplicas == 0 {
		return nil, errors.New("spec.add size is nil or target replicas is zero")
	}
	if len(r.Spec.Minus.ScaleTime) == 0 {
		return nil, errors.New("spec.minus size is nil")
	}
	if r.Spec.DeploymentName == "" {
		return nil, errors.New("spec.deploymentName is empty.it will use nginx")
	}
	if len(r.Spec.ImagePullTime) == 0 {
		return nil, errors.New("spec.imagePull time  is empty")
	}

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
