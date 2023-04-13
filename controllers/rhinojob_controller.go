/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kbatchv1 "k8s.io/api/batch/v1"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rhinooprapiv1alpha1 "github.com/OpenRHINO/RHINO-Operator/api/v1alpha1" //这里导入后的名字跟脚手架代码自动生成的不一样，这样改的原因是为了可读性更好
)

// RhinoJobReconciler reconciles a RhinoJob object
type RhinoJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type LauncherPodStatus int

const (
	Terminated LauncherPodStatus = iota
	ContainerCreating
	ImageError
	Running
)

//+kubebuilder:rbac:groups=openrhino.org,resources=rhinojobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openrhino.org,resources=rhinojobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openrhino.org,resources=rhinojobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RhinoJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.1/pkg/reconcile
func (r *RhinoJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("In the reconcile function of RhinoJob.")

	var rhinojob rhinooprapiv1alpha1.RhinoJob
	if err := r.Get(ctx, req.NamespacedName, &rhinojob); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get RhinoJob")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var foundLauncherJob kbatchv1.Job
	var foundWorkersJob kbatchv1.Job
	var imagePullState LauncherPodStatus
	// 获取该 RhinoJob 对应的 MPI launcher Job 和 workers job，若都不存在，则创建它们
	// 若只有一个不存在，另一个正常运行，则不做任何处理，因为这种情况只有三种可能：
	// 1. 刚要创建某个job，还查不到
	// 2. 故障导致上次该job创建失败
	// 3. 因人为、故障或其他原因导致删除。
	// 这三种可能都不应当重新触发创建流程：对于情况1，让它继续运行就好；情况2 或 情况3，即一个如果没了，只有另一个在，重新创建出来也意义也不大，大概率是连不上，或者任务仍然会失败，
	// 然而如果人为把这另一个job也删掉，作为修复故障的手段是OK的，这就是让这个rhinojob重新跑
	errGetLauncherJob := r.Get(ctx, types.NamespacedName{Namespace: rhinojob.Namespace, Name: nameForLauncherJob(&rhinojob)}, &foundLauncherJob)
	errGetWorkersJob := r.Get(ctx, types.NamespacedName{Namespace: rhinojob.Namespace, Name: nameForWorkersJob(&rhinojob)}, &foundWorkersJob)

	if errors.IsNotFound(errGetLauncherJob) && errors.IsNotFound(errGetWorkersJob) { // Launcher Job 和 workers Job 都不存在
		// 根据获取的 RhinoJob，构造Launcher Job
		launcherJob, err := r.constructLauncherJob(&rhinojob)
		if err != nil {
			return ctrl.Result{}, err
		}
		// 创建Launcher Job
		if err := r.Create(ctx, launcherJob); err != nil && !errors.IsAlreadyExists(err) {
			//若Launcher Job 创建失败，而且失败原因也不是“资源已存在”，报错并返回错误
			logger.Error(err, "Unable to create launcher job for RhinoJob", "Job", launcherJob)
			return ctrl.Result{}, err
		}
		logger.Info("Launcher job created", "job", launcherJob)
	}
	if errGetLauncherJob != nil && !errors.IsNotFound(errGetLauncherJob) { // Launcher Job 获取失败，且原因也不是“该资源不存在”
		logger.Error(errGetLauncherJob, "Failed to get launcher job")
		return ctrl.Result{}, errGetLauncherJob
	}

	if errors.IsNotFound(errGetWorkersJob) {
		launcherPodLabels := labelsForLauncherPod(&rhinojob)
		var foundPodList kcorev1.PodList
		if err := r.List(ctx, &foundPodList, client.MatchingLabels(launcherPodLabels)); err != nil {
			logger.Error(err, "Unable to find pod list")
			return ctrl.Result{}, err
		}
		imagePullState = checkLauncherPodWithImageError(&foundPodList)
		if len(foundPodList.Items) != 0 && foundPodList.Items[0].Status.Phase == "Running" {
			podStatus := foundPodList.Items[0].Status.Conditions[1].Status
			if podStatus == "True" {
				//构造 Workers Job
				workersJob, err := r.constructWorkersJob(&rhinojob, ctx)
				if err != nil {
					return ctrl.Result{}, err
				}
				if err := r.Create(ctx, workersJob); err != nil && !errors.IsAlreadyExists(err) {
					logger.Error(err, "Unable to create workers job for RhinoJob", "Job", workersJob)
					return ctrl.Result{}, err
				}
				logger.Info("Workers job created", "job", workersJob)
				return ctrl.Result{}, nil
			}
		}
	}
	if errGetWorkersJob != nil && !errors.IsNotFound(errGetWorkersJob) { //Workers Job 获取失败，且原因也不是“该资源不存在”
		logger.Error(errGetWorkersJob, "Failed to get workers job")
		return ctrl.Result{}, errGetWorkersJob
	}

	// 更新 status
	if errGetLauncherJob != nil || errGetWorkersJob != nil {
		if imagePullState == ImageError {
			rhinojob.Status.JobStatus = rhinooprapiv1alpha1.ImageError
		} else {
			rhinojob.Status.JobStatus = rhinooprapiv1alpha1.Pending
		}
	} else {
		if foundWorkersJob.Status.Failed+foundLauncherJob.Status.Failed > 0 {
			rhinojob.Status.JobStatus = rhinooprapiv1alpha1.Failed
		} else if foundWorkersJob.Status.Succeeded == *rhinojob.Spec.Parallelism && foundLauncherJob.Status.Succeeded == 1 {
			rhinojob.Status.JobStatus = rhinooprapiv1alpha1.Completed
		} else {
			rhinojob.Status.JobStatus = rhinooprapiv1alpha1.Running
		}
	}
	if err := r.Status().Update(ctx, &rhinojob); err != nil {
		logger.Error(err, "Failed to update RhinoJob status")
		return ctrl.Result{}, err
	}

	// 如果 Launcher Job 正在拉取镜像，等待 3 秒后 Requeue
	if imagePullState == ContainerCreating {
		logger.Info("Waiting for Launcher job", "State", "ContainerCreating")
		return ctrl.Result{RequeueAfter: time.Second * 3}, nil
	}

	// 处理 TTL
	if *rhinojob.Spec.TTL > 0 {
		ttl_left := rhinojob.CreationTimestamp.Add(time.Second * time.Duration(*rhinojob.Spec.TTL)).Sub(time.Now())
		if ttl_left > 0 {
			return ctrl.Result{RequeueAfter: ttl_left}, nil
		} else {
			r.Delete(ctx, &rhinojob)
		}
	}

	return ctrl.Result{}, nil
}

// 检查 Launcher Pod 是否存在 ImagePullBackOff 或者 ErrImagePull 的错误
func checkLauncherPodWithImageError(PodList *kcorev1.PodList) LauncherPodStatus {
	if len(PodList.Items) != 0 {
		for _, containerStatus := range PodList.Items[0].Status.ContainerStatuses {
			switch {
			case containerStatus.State.Waiting != nil:
				// Launcher Pod 处于 Waiting 状态，检查是否是因为镜像拉取失败
				if containerStatus.State.Waiting.Reason == "ContainerCreating" {
					return ContainerCreating
				} else {
					return ImageError
				}
			case containerStatus.State.Running != nil:
				// Launcher Pod 创建成功，正在运行
				return Running
			case containerStatus.State.Terminated != nil:
				// Launcher Pod 已经完成
				return Terminated
			}
		}
	}
	return ContainerCreating
}

// Launcher Job 的名称
func nameForLauncherJob(rj *rhinooprapiv1alpha1.RhinoJob) string {
	return rj.Name + "-" + string(rj.UID)[:5] + "-launcher"
}

// Launcher Pod 的 Labels，也是后续寻址 Launcher Pod 的 Selector
func labelsForLauncherPod(rj *rhinooprapiv1alpha1.RhinoJob) map[string]string {
	return map[string]string{"app": rj.Name, "type": "launcher", "rhinoID": string(rj.UID)}
}

func nameForWorkersJob(rj *rhinooprapiv1alpha1.RhinoJob) string {
	return rj.Name + "-" + string(rj.UID)[:5] + "-workers"
}

// 构造Launcher Job
func (r *RhinoJobReconciler) constructLauncherJob(rj *rhinooprapiv1alpha1.RhinoJob) (*kbatchv1.Job, error) {
	name := nameForLauncherJob(rj)
	launcherPodLabels := labelsForLauncherPod(rj)

	// 构造 Launcher 进程的命令行
	hostsIDs := "'0"
	for i := 1; i < int(*rj.Spec.Parallelism); i++ {
		hostsIDs = fmt.Sprintf("%s,%d", hostsIDs, i)
	}
	hostsIDs = hostsIDs + "'" // 需要加上单引号，否则 mpirun 会报错，hostsIDs = "'0,1,2,3'" for --np=4
	cmdArgs := append([]string{"mpirun", "-launcher", "manual", "-verbose", "-disable-hostname-propagation", "-hosts", hostsIDs,
		rj.Spec.AppExec}, rj.Spec.AppArgs...)
	cmdArgs = append(cmdArgs, "|", "tee", "/tmp/rhino-launcher.log")
	cmdArgString := strings.Join(cmdArgs, " ")

	// 构造 Launcher Job
	job := &kbatchv1.Job{
		ObjectMeta: kmetav1.ObjectMeta{
			Name:      name,
			Namespace: rj.Namespace,
		},
		Spec: kbatchv1.JobSpec{
			Template: kcorev1.PodTemplateSpec{
				ObjectMeta: kmetav1.ObjectMeta{
					Labels: launcherPodLabels,
				},
				Spec: kcorev1.PodSpec{
					Containers: []kcorev1.Container{{
						Image:   rj.Spec.Image,
						Name:    "rhino-mpi-launcher",
						Command: []string{"/bin/sh", "-c"},
						Args:    []string{cmdArgString},
						ReadinessProbe: &kcorev1.Probe{
							ProbeHandler: kcorev1.ProbeHandler{
								Exec: &kcorev1.ExecAction{
									Command: []string{"/bin/sh", "-c", "cat /tmp/rhino-launcher.log | grep HYDRA_LAUNCH_END"},
								},
							},
							InitialDelaySeconds: 1,
							PeriodSeconds:       1,
							FailureThreshold:    1,
						},
					}},
					RestartPolicy: "OnFailure",
				},
			},
		},
	}
	err := ctrl.SetControllerReference(rj, job, r.Scheme)

	return job, err
}

// 构造 Workers Job
func (r *RhinoJobReconciler) constructWorkersJob(rj *rhinooprapiv1alpha1.RhinoJob, ctx context.Context) (*kbatchv1.Job, error) {
	name := nameForWorkersJob(rj)
	launcherPodLabels := labelsForLauncherPod(rj)

	var foundPodList kcorev1.PodList
	r.List(ctx, &foundPodList, client.MatchingLabels(launcherPodLabels))
	completionMode := "Indexed"
	cmdArgs := []string{"-c", "/usr/local/bin/hydra_pmi_proxy --control-port " + foundPodList.Items[0].Status.PodIP +
		":20000 --debug --rmk user --launcher manual --demux poll --pgid 0 --retries 10 --usize -2 --proxy-id $JOB_COMPLETION_INDEX"}

	// 构造 Workers Job
	job := &kbatchv1.Job{
		ObjectMeta: kmetav1.ObjectMeta{
			Name:      name,
			Namespace: rj.Namespace,
		},
		Spec: kbatchv1.JobSpec{
			CompletionMode: (*kbatchv1.CompletionMode)(&completionMode),
			Completions:    rj.Spec.Parallelism,
			Parallelism:    rj.Spec.Parallelism,
			Template: kcorev1.PodTemplateSpec{
				Spec: kcorev1.PodSpec{
					Containers: []kcorev1.Container{{
						Image:   rj.Spec.Image,
						Name:    "rhino-mpi-worker",
						Command: []string{"ash"},
						Args:    cmdArgs,
					}},
					RestartPolicy: "Never",
				},
			},
		},
	}

	if rj.Spec.DataPath != "" && rj.Spec.DataServer != "" {
		job.Spec.Template.Spec.Volumes = []kcorev1.Volume{{
			Name: "data",
			VolumeSource: kcorev1.VolumeSource{
				NFS: &kcorev1.NFSVolumeSource{
					Server: rj.Spec.DataServer,
					Path:   rj.Spec.DataPath,
				},
			},
		}}
		job.Spec.Template.Spec.Containers[0].VolumeMounts = []kcorev1.VolumeMount{{
			MountPath: "/data",
			Name:      "data",
		}}
	}

	err := ctrl.SetControllerReference(rj, job, r.Scheme)

	return job, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *RhinoJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rhinooprapiv1alpha1.RhinoJob{}).
		Owns(&kbatchv1.Job{}).
		Complete(r)
}
