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

package controller

import (
	"context"
	"fmt"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	applicationv1 "liuchong.cn/m/api/v1"
	"liuchong.cn/m/module"
	"liuchong.cn/m/pkg"
	"liuchong.cn/m/pkg/cron"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

var (
	// crontab 保存所有 CronScale 资源注册出来的定时任务。
	crontab = pkg.CronClient()
	// clientset 用于直接操作 Deployment、HPA、Node、Job 等原生资源。
	clientset = pkg.KubectlClient()
	// db 保存 agent 上报的节点镜像信息，用于计算镜像预热节点列表。
	db = pkg.InitMySQL()
	// taskMap 记录 CronScale 与 cron task id 的关系，资源删除时需要清理任务。
	taskMap = map[string][]cron.EntryID{}
)

// CronScaleReconciler reconciles a CronScale object
type CronScaleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=application.liuchong.cn,resources=cronscales,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=application.liuchong.cn,resources=cronscales/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=application.liuchong.cn,resources=cronscales/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CronScale object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.2/pkg/reconcile
func (r *CronScaleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("Triggering CronScale reconciliation.............")
	// 获取当前被调协的 CronScale 资源。
	app := applicationv1.CronScale{}
	if err := r.Client.Get(ctx, req.NamespacedName, &app); err != nil {
		if errors.IsNotFound(err) {
			// CronScale 被删除后，移除之前注册的定时任务，避免后台任务继续执行。
			for taskKey, taskvalue := range taskMap {
				if taskKey == req.Namespace+"/"+req.Name {
					for _, value := range taskvalue {
						crontab.Remove(value)
					}
					l.Info("Triggering CronScale delete.................", "Name:", taskKey)
					return ctrl.Result{}, nil
				}
			}
			//defer crontab.Stop()
			l.Info("The CronScale is not found")
			return ctrl.Result{}, nil
		}
		l.Error(err, "failed to get the CronScale object")
		return ctrl.Result{RequeueAfter: time.Second * 30}, err
	}
	_, err := r.ReconcileReplicas(ctx, req, app)
	if err != nil {
		l.Error(err, "failed to reconcile replicas")
		return ctrl.Result{RequeueAfter: time.Second * 30}, err
	}
	_, err = r.ReconcileImage(ctx, req, app)
	if err != nil {
		l.Error(err, "failed to reconcile image")
		return ctrl.Result{RequeueAfter: time.Second * 30}, err
	}
	crontab.Start()
	// TODO(user): your logic here
	return ctrl.Result{}, nil
}

// ReconcileReplicas
// 1.获取到关联到deploy的信息（名称、副本数） ✅
// 2.获取到CronScale资源配置的期望副本数 ✅
// 3.到达指定时间后，触发调协逻辑 ✅
// 调协逻辑：判断CronScale的副本数和deploy的副本数是否相同，相同则直接返回即可，不相同则调整到与CronScale相同的副本数 ✅
func (r *CronScaleReconciler) ReconcileReplicas(ctx context.Context, req ctrl.Request, app applicationv1.CronScale) (ctrl.Result, error) {
	var taskAdd = taskAdd{}
	var taskMinus = taskMinus{}
	// 注册扩容任务，到 spec.add.scaleTime 后执行 taskAdd.Run。
	{
		scaleTime := app.Spec.Add.ScaleTime
		addID, _ := crontab.AddJob(scaleTime, &taskAdd, ctx, req, app, clientset)
		taskMap[req.Namespace+"/"+req.Name] = append(taskMap[req.Namespace+"/"+req.Name], addID)
	}
	// 注册缩容任务，到 spec.minus.scaleTime 后执行 taskMinus.Run。
	{
		scaleTime := app.Spec.Minus.ScaleTime
		minusID, _ := crontab.AddJob(scaleTime, &taskMinus, ctx, req, app, clientset)
		taskMap[req.Namespace+"/"+req.Name] = append(taskMap[req.Namespace+"/"+req.Name], minusID)
	}

	// TODO(user): your logic here
	return ctrl.Result{}, nil
}

// ReconcileImage 镜像预热功能实现
// 1.获取add模块 deployment实例的镜像信息 ✅
// 2.判断节点是否存在镜像信息 ✅
// 3.不存在则创建一个带有索引类型的job，携带pod反亲和的硬限制，去每个机器拉取镜像信息  （本地测试✅）
// 4.拉取完毕后，去查询job状态，当状态时Completed，镜像预热结束 ✅
// 5.回收的时机考虑：每隔五秒去查询结果 ✅
// 6.查询到完成，调协结束 ✅
func (r *CronScaleReconciler) ReconcileImage(ctx context.Context, req ctrl.Request, app applicationv1.CronScale) (ctrl.Result, error) {
	var taskImage = taskImage{}
	// 注册镜像预热任务，到 spec.imagePullTime 后执行 taskImage.Run。
	pullTime := app.Spec.ImagePullTime
	imageID, _ := crontab.AddJob(pullTime, &taskImage, ctx, req, app, clientset)
	taskMap[req.Namespace+"/"+req.Name] = append(taskMap[req.Namespace+"/"+req.Name], imageID)
	// TODO(user): your logic here
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CronScaleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&applicationv1.CronScale{}).
		Complete(r)
}

type taskAdd struct{}

func (t *taskAdd) Run(ctx context.Context, req ctrl.Request, app applicationv1.CronScale, clientset *kubernetes.Clientset) {
	l := log.FromContext(ctx)
	l.Info("Cron AddTask Running....", "Get DeployInfo", time.Now())
	deploy, err := clientset.AppsV1().Deployments(req.Namespace).Get(ctx, app.Spec.DeploymentName, metav1.GetOptions{})
	if err != nil {
		l.Error(err, "failed to get the deployment")
		return
	}
	// 如果业务配置了 HPA，优先调整 HPA 副本上下限，避免 HPA 覆盖手动扩容结果。
	{
		minReplicas, err := pkg.MinHpaReplicas(ctx, clientset, app.Namespace, deploy.Name)
		if err != nil {
			l.Error(err, "failed to get the minReplicas", "minReplicas", minReplicas)
		} else {
			if app.Spec.Add.TargetReplicas <= minReplicas {
				l.Info("app.spec.add.TargetReplicas It HpaminReplicas or equal HpaminReplicas", "minReplicas", minReplicas)
				return
			}
			if err = pkg.UpdateMinHpaReplicas(ctx, clientset, app.Namespace, deploy.Name, app.Spec.Add.TargetReplicas); err != nil {
				l.Error(err, "failed to update the minReplicas", "minReplicas", minReplicas)
				return
			}
		}
	}

	// 没有 HPA 或 HPA 处理后，再通过 Scale 子资源调整 Deployment 副本数。
	if app.Spec.Add.TargetReplicas != *deploy.Spec.Replicas {
		scaleInfo, err := clientset.AppsV1().Deployments(req.Namespace).GetScale(ctx, app.Spec.DeploymentName, metav1.GetOptions{})
		if err != nil {
			l.Error(err, "failed to get the deployment scale")
			return
		}
		scaleInfo.Spec.Replicas = app.Spec.Add.TargetReplicas
		scaleInfo, err = clientset.AppsV1().Deployments(req.Namespace).UpdateScale(ctx, app.Spec.DeploymentName, scaleInfo, metav1.UpdateOptions{})
		if err != nil {
			l.Error(err, "failed to update the deployment scale")
		}
		l.Info("Cron AddTask done....", "Get DeployInfo", time.Now())
		return
	}
	l.Info("Cronscale resource replicas is same count ", "name:", req.Namespace+"/"+req.Name)

}

type taskMinus struct{}

func (t *taskMinus) Run(ctx context.Context, req ctrl.Request, app applicationv1.CronScale, clientset *kubernetes.Clientset) {
	l := log.FromContext(ctx)
	l.Info("Cron MinusTask Running....", "Get DeployInfo", time.Now())
	deploy, err := clientset.AppsV1().Deployments(req.Namespace).Get(ctx, app.Spec.DeploymentName, metav1.GetOptions{})
	if err != nil {
		l.Error(err, "failed to get the deployment")
		return
	}
	// 如果业务配置了 HPA，缩容时同样优先调整 HPA 副本上下限。
	{
		minReplicas, err := pkg.MinHpaReplicas(ctx, clientset, app.Namespace, deploy.Name)
		if err != nil {
			l.Error(err, "failed to get the minReplicas", "minReplicas", minReplicas)
		} else {
			if app.Spec.Minus.TargetReplicas >= minReplicas {
				l.Info("app.spec.minus.TargetReplicas gt  HpaminReplicas or equal HpaminReplicas", "minReplicas", minReplicas)
				return
			}
			if err = pkg.UpdateMinHpaReplicas(ctx, clientset, app.Namespace, deploy.Name, app.Spec.Minus.TargetReplicas); err != nil {
				l.Error(err, "failed to update the minReplicas", "minReplicas", minReplicas)
				return
			}
		}
	}
	// 没有 HPA 或 HPA 处理后，再通过 Scale 子资源调整 Deployment 副本数。
	if app.Spec.Minus.TargetReplicas != *deploy.Spec.Replicas {
		scaleInfo, err := clientset.AppsV1().Deployments(req.Namespace).GetScale(ctx, app.Spec.DeploymentName, metav1.GetOptions{})
		if err != nil {
			l.Error(err, "failed to get the deployment scale")
			return
		}
		scaleInfo.Spec.Replicas = app.Spec.Minus.TargetReplicas
		scaleInfo, err = clientset.AppsV1().Deployments(req.Namespace).UpdateScale(ctx, app.Spec.DeploymentName, scaleInfo, metav1.UpdateOptions{})
		if err != nil {
			l.Error(err, "failed to update the deployment scale")
		}
		l.Info("Cron MinusTask done....", "Get DeployInfo", time.Now())
		return
	}
	l.Info("Cronscale resource replicas is same count ", "name:", req.Namespace+"/"+req.Name)

}

type taskImage struct{}

func (t *taskImage) Run(ctx context.Context, req ctrl.Request, app applicationv1.CronScale, clientset *kubernetes.Clientset) {
	l := log.FromContext(ctx)
	oldnodelist := make([]string, 0, 30)
	nodelist := make([]string, 0, 30)
	deploy, err := clientset.AppsV1().Deployments(req.Namespace).Get(ctx, app.Spec.DeploymentName, metav1.GetOptions{})
	if err != nil {
		l.Error(err, "failed to get the deployment")
		return
	}
	deployImage := deploy.Spec.Template.Spec.Containers[0].Image
	// 获取集群所有节点，后续和 MySQL 中已有镜像的节点做差集。
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	for _, node := range nodes.Items {
		if err != nil {
			l.Error(err, "failed to get the node")
			return
		}
		oldnodelist = append(oldnodelist, node.Name)
	}
	// 查询已经存在目标镜像的节点，只对缺少镜像的节点创建预热 Job。
	{
		var nodeInfo []module.NodeInfo
		sqlStr := "select node_ip from cronscale_agent where image_name LIKE CONCAT('%',?) "
		if err = db.Select(&nodeInfo, sqlStr, deployImage); err != nil {
			l.Error(err, "failed to get the nodeInfo from mysql")
			return
		}
		// 数据库没有命中时，表示所有节点都需要预热。
		if len(nodeInfo) == 0 {
			nodelist = make([]string, len(oldnodelist))
			copy(nodelist, oldnodelist)
		} else {
			// 遍历所有节点，筛出没有目标镜像的节点。
			for _, nodeName := range oldnodelist {
				for nodeImageindex, nodeImage := range nodeInfo {
					if nodeName == nodeImage.NodeIp {
						break
					}
					// 遍历到最后仍未匹配，说明该节点需要预热。
					if nodeName != nodeImage.NodeIp && nodeImageindex == len(nodeInfo)-1 {
						nodelist = append(nodelist, nodeName)
					}
				}
			}
		}
	}
	l.Info("oldnodelist(TKE的所有节点列表):", "value", oldnodelist)
	l.Info("nodelist(需要预热的节点列表):", "value", nodelist)
	if len(nodelist) == 0 {
		l.Info("节点无需镜像预热", "now", time.Now())
		return
	}
	// 每个待预热节点对应一个 Job Pod，保证所有缺镜像节点并发拉取。
	completions := int32(len(nodelist))
	parallelism := int32(len(nodelist))
	randStr := pkg.RandStringBytes()
	propagationPolicy := "Background"
	job, err := clientset.BatchV1().Jobs(req.Namespace).Create(ctx, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cronscale-task-%s", randStr),
			Namespace: req.Namespace,
		},
		Spec: batchv1.JobSpec{
			Completions: &completions,
			Parallelism: &parallelism,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Affinity: &v1.Affinity{
						PodAntiAffinity: &v1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "job-name",
												Operator: "In",
												Values:   []string{fmt.Sprintf("cronscale-task-%s", randStr)},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
						NodeAffinity: &v1.NodeAffinity{
							// 只调度到缺少目标镜像的节点。
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/hostname",
												Operator: "In",
												Values:   nodelist,
											},
										},
									},
								},
							},
						},
					},
					RestartPolicy: "Never",
					Containers: []v1.Container{
						{
							Name:    "cronscale-task",
							Image:   "example.com/library/cronscale-task:3.16",
							Command: []string{"/bin/sh"},
							// 在目标节点执行 crictl pull，提前把业务镜像拉到本地。
							Args: []string{"-c", fmt.Sprintf("crictl pull %s;echo pull done....", deployImage)},
							VolumeMounts: []v1.VolumeMount{
								{Name: "containerd-data", MountPath: "/data/containerd"},
								{Name: "containerd-socket", MountPath: "/run/containerd"},
								{Name: "crictl", MountPath: "/usr/local/bin/crictl"},
								{Name: "crictl-conf", MountPath: "/etc/crictl.yaml"},
							},
						},
					},
					Volumes: []v1.Volume{
						{Name: "containerd-data", VolumeSource: v1.VolumeSource{
							HostPath: &v1.HostPathVolumeSource{
								Path: "/data/containerd",
							},
						},
						},
						{Name: "containerd-socket", VolumeSource: v1.VolumeSource{
							HostPath: &v1.HostPathVolumeSource{
								Path: "/run/containerd",
							},
						},
						},
						{Name: "crictl", VolumeSource: v1.VolumeSource{
							HostPath: &v1.HostPathVolumeSource{
								Path: "/usr/local/bin/crictl",
							},
						},
						},
						{Name: "crictl-conf", VolumeSource: v1.VolumeSource{
							HostPath: &v1.HostPathVolumeSource{
								Path: "/etc/crictl.yaml",
							},
						},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		l.Error(err, "failed to create the job")
		return
	}
	// 轮询 Job 状态，完成后删除临时 Job，避免资源残留。
	for {
		<-time.NewTicker(time.Second * 5).C
		job, err = clientset.BatchV1().Jobs(req.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
		if err != nil {
			l.Error(err, "failed to get the job")
			continue
		}
		if job.Status.Conditions == nil {
			continue
		}
		if job.Status.Conditions[0].Type == "Complete" && job.Status.Conditions[0].Status == "True" {
			if err = clientset.BatchV1().Jobs(req.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{PropagationPolicy: (*metav1.DeletionPropagation)(&propagationPolicy)}); err != nil {
				l.Error(err, "failed to delete the job")
				continue
			}
			l.Info("镜像预热完成", "now", time.Now())
			l.Info("清理Job Task完成", "job", job.Name)
			break
		}
	}

}
