/*
Copyright 2023.

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

package controller

import (
	"context"
	"strconv"

	updatev1beta1 "github.com/git-ogawa/docserver/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsv1apply "k8s.io/client-go/applyconfigurations/apps/v1"
	batchv1apply "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DocServerReconciler reconciles a DocServer object
type DocServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=update.git-ogawa.github.io,resources=docservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=update.git-ogawa.github.io,resources=docservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=update.git-ogawa.github.io,resources=docservers/finalizers,verbs=update

//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DocServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *DocServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var ds updatev1beta1.DocServer
	err := r.Get(ctx, req.NamespacedName, &ds)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		logger.Error(err, "unable to get DocServer", "name", req.NamespacedName)
		return ctrl.Result{}, err
	}

	if !ds.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	err = r.reconcilePersistenVolumeClaim(ctx, ds)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileJob(ctx, ds)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileDeployment(ctx, ds)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.reconcileService(ctx, ds)
	if err != nil {
		return ctrl.Result{}, err
	}

	return r.updateStatus(ctx, ds)
}

func (r *DocServerReconciler) reconcileJob(ctx context.Context, ds updatev1beta1.DocServer) error {
	logger := log.FromContext(ctx)

	jobName := "gitpod-" + ds.Name
	pvcName := "docserver-" + ds.Name

	gitUrl := ds.Spec.Target.Url
	branch := "main"
	if len(ds.Spec.Target.Branch) != 0 {
		branch = ds.Spec.Target.Branch
	}

	depth := 1
	if ds.Spec.Target.Depth != 1 {
		depth = ds.Spec.Target.Depth
	}

	sslVerify := true
	if ds.Spec.Target.SSLVerify != nil {
		sslVerify = *ds.Spec.Target.SSLVerify
	}

	image := "docogawa/gitpod:latest"
	if len(ds.Spec.Gitpod.Image) != 0 {
		image = ds.Spec.Gitpod.Image
	}

	owner, err := controllerReference(ds, r.Scheme)
	if err != nil {
		return err
	}

	job := batchv1apply.Job(jobName, ds.Namespace).
		WithLabels(map[string]string{
			"app.kubernetes.io/name":       "mkdocs",
			"app.kubernetes.io/instance":   ds.Name,
			"app.kubernetes.io/created-by": "docserver-controller",
		}).
		WithOwnerReferences(owner).
		WithSpec(batchv1apply.JobSpec().
			WithBackoffLimit(5).
			WithCompletions(1).
			WithTemplate(corev1apply.PodTemplateSpec().
				WithLabels(map[string]string{
					"app.kubernetes.io/name":       "mkdocs",
					"app.kubernetes.io/instance":   ds.Name,
					"app.kubernetes.io/created-by": "docserver-controller",
				}).
				WithSpec(corev1apply.PodSpec().
					WithContainers(corev1apply.Container().
						WithName("gitpod").
						WithImage(image).
						WithImagePullPolicy(corev1.PullIfNotPresent).
						WithVolumeMounts(corev1apply.VolumeMount().
							WithName("source").
							WithMountPath("/docs"),
						).
						WithEnv(
							corev1apply.EnvVar().
								WithName("GIT_URL").
								WithValue(gitUrl),
							corev1apply.EnvVar().
								WithName("GIT_BRANCH").
								WithValue(branch),
							corev1apply.EnvVar().
								WithName("GIT_DEPTH").
								WithValue(strconv.Itoa(depth)),
							corev1apply.EnvVar().
								WithName("GIT_SSL_VERIFY").
								WithValue(strconv.FormatBool(sslVerify)),
						),
					).
					WithRestartPolicy(corev1.RestartPolicyNever).
					WithVolumes(corev1apply.Volume().
						WithName("source").
						WithPersistentVolumeClaim(corev1apply.PersistentVolumeClaimVolumeSource().
							WithClaimName(pvcName),
						),
					),
				),
			),
		)

	if len(ds.Spec.Target.BasicAuthSecret) != 0 {
		basicAuthSecret := ds.Spec.Target.BasicAuthSecret
		envVars := []corev1apply.EnvVarApplyConfiguration{
			*corev1apply.EnvVar().
				WithName("GIT_USERNAME").
				WithValueFrom(corev1apply.EnvVarSource().
					WithSecretKeyRef(corev1apply.SecretKeySelector().
						WithName(basicAuthSecret).
						WithKey("username"),
					),
				),
			*corev1apply.EnvVar().
				WithName("GIT_PASSWORD").
				WithValueFrom(corev1apply.EnvVarSource().
					WithSecretKeyRef(corev1apply.SecretKeySelector().
						WithName(basicAuthSecret).
						WithKey("password"),
					),
				),
		}
		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, envVars...)
	}

	if ds.Spec.Target.SSHSecret != nil {
		sshConfig := ds.Spec.Target.SSHSecret.Config
		privateKey := ds.Spec.Target.SSHSecret.PrivateKey
		volumeMounts := []corev1apply.VolumeMountApplyConfiguration{
			*corev1apply.VolumeMount().
				WithName("sshconfig").
				WithMountPath("/opt/gitpod/sshconfig").
				WithReadOnly(true),
			*corev1apply.VolumeMount().
				WithName("privatekey").
				WithMountPath("/opt/gitpod/privatekey").
				WithReadOnly(true),
		}
		volumes := []corev1apply.VolumeApplyConfiguration{
			*corev1apply.Volume().
				WithName("sshconfig").
				WithConfigMap(corev1apply.ConfigMapVolumeSource().
					WithName(sshConfig),
				),
			*corev1apply.Volume().
				WithName("privatekey").
				WithSecret(corev1apply.SecretVolumeSource().
					WithSecretName(privateKey),
				),
		}
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMounts...)
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, volumes...)
	}

	if len(ds.Spec.Target.TLSSecret) != 0 {
		tlsSecret := ds.Spec.Target.TLSSecret
		volumeMount := corev1apply.VolumeMount().
			WithName("cacert").
			WithMountPath("/opt/gitpod/certs").
			WithReadOnly(true)
		volume := corev1apply.Volume().
			WithName("cacert").
			WithSecret(corev1apply.SecretVolumeSource().
				WithSecretName(tlsSecret),
			)
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, *volumeMount)
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, *volume)
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(job)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var current batchv1.Job
	err = r.Get(ctx, client.ObjectKey{Namespace: ds.Namespace, Name: jobName}, &current)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	currApplyConfig, err := batchv1apply.ExtractJob(&current, "docserver-controller")
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(job, currApplyConfig) {
		return nil
	}

	err = r.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "docserver-controller",
		Force:        pointer.Bool(true),
	})

	if err != nil {
		logger.Error(err, "unable to create or update Job")
		return err
	}
	logger.Info("reconcile Job successfully", "name", ds.Name)
	return nil
}

func (r *DocServerReconciler) reconcilePersistenVolumeClaim(ctx context.Context, ds updatev1beta1.DocServer) error {
	logger := log.FromContext(ctx)

	pvcName := "docserver-" + ds.Name

	size := "3Gi"
	if len(ds.Spec.Storage.Size) != 0 {
		size = ds.Spec.Storage.Size
	}

	storageClassName := "default"
	if len(ds.Spec.Storage.StorageClass) != 0 {
		storageClassName = ds.Spec.Storage.StorageClass
	}

	accessmode := corev1.ReadWriteMany

	owner, err := controllerReference(ds, r.Scheme)
	if err != nil {
		return err
	}
	blockOwnerDeletion := false
	owner.BlockOwnerDeletion = &blockOwnerDeletion
	if ds.Spec.Storage.BlockOwnerDeletion != nil {
		owner.BlockOwnerDeletion = ds.Spec.Storage.BlockOwnerDeletion
	}

	pvc := corev1apply.PersistentVolumeClaim(pvcName, ds.Namespace).
		WithLabels(map[string]string{
			"app.kubernetes.io/name":       "mkdocs",
			"app.kubernetes.io/instance":   ds.Name,
			"app.kubernetes.io/created-by": "docserver-controller",
		}).
		WithOwnerReferences(owner).
		WithSpec(corev1apply.PersistentVolumeClaimSpec().
			WithResources(corev1apply.ResourceRequirements().
				WithRequests(corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				}),
			).
			WithAccessModes(accessmode).
			WithStorageClassName(storageClassName).
			WithVolumeMode(corev1.PersistentVolumeFilesystem),
		)

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pvc)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var current corev1.PersistentVolumeClaim
	err = r.Get(ctx, client.ObjectKey{Namespace: ds.Namespace, Name: pvcName}, &current)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	currApplyConfig, err := corev1apply.ExtractPersistentVolumeClaim(&current, "docserver-controller")
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(pvc, currApplyConfig) {
		return nil
	}

	err = r.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "docserver-controller",
		Force:        pointer.Bool(true),
	})
	if err != nil {
		logger.Error(err, "unable to create or update PersistentVolumeClaim")
		return err
	}

	logger.Info("reconcile PersistentVolumeClaim successfully", "name", ds.Name)
	return nil
}

func (r *DocServerReconciler) reconcileDeployment(ctx context.Context, ds updatev1beta1.DocServer) error {
	logger := log.FromContext(ctx)

	depName := "docserver-" + ds.Name
	pvcName := "docserver-" + ds.Name

	image := "squidfunk/mkdocs-material"
	if len(ds.Spec.Image) != 0 {
		image = ds.Spec.Image
	}
	owner, err := controllerReference(ds, r.Scheme)
	if err != nil {
		return err
	}

	dep := appsv1apply.Deployment(depName, ds.Namespace).
		WithLabels(map[string]string{
			"app.kubernetes.io/name":       "mkdocs",
			"app.kubernetes.io/instance":   ds.Name,
			"app.kubernetes.io/created-by": "docserver-controller",
		}).
		WithOwnerReferences(owner).
		WithSpec(appsv1apply.DeploymentSpec().
			WithReplicas(ds.Spec.Replicas).
			WithSelector(metav1apply.LabelSelector().WithMatchLabels(map[string]string{
				"app.kubernetes.io/name":       "mkdocs",
				"app.kubernetes.io/instance":   ds.Name,
				"app.kubernetes.io/created-by": "docserver-controller",
			})).
			WithTemplate(corev1apply.PodTemplateSpec().
				WithLabels(map[string]string{
					"app.kubernetes.io/name":       "mkdocs",
					"app.kubernetes.io/instance":   ds.Name,
					"app.kubernetes.io/created-by": "docserver-controller",
				}).
				WithSpec(corev1apply.PodSpec().
					WithContainers(corev1apply.Container().
						WithName("mkdocs").
						WithImage(image).
						WithImagePullPolicy(corev1.PullIfNotPresent).
						WithVolumeMounts(corev1apply.VolumeMount().
							WithName("source").
							WithMountPath("/docs"),
						).
						WithPorts(corev1apply.ContainerPort().
							WithName("http").
							WithProtocol(corev1.ProtocolTCP).
							WithContainerPort(8000),
						).
						WithLivenessProbe(corev1apply.Probe().
							WithHTTPGet(corev1apply.HTTPGetAction().
								WithPort(intstr.FromString("http")).
								WithPath("/").
								WithScheme(corev1.URISchemeHTTP),
							),
						).
						WithReadinessProbe(corev1apply.Probe().
							WithHTTPGet(corev1apply.HTTPGetAction().
								WithPort(intstr.FromString("http")).
								WithPath("/").
								WithScheme(corev1.URISchemeHTTP),
							),
						),
					).
					WithVolumes(corev1apply.Volume().
						WithName("source").
						WithPersistentVolumeClaim(corev1apply.PersistentVolumeClaimVolumeSource().
							WithClaimName(pvcName),
						),
					),
				),
			),
		)

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dep)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var current appsv1.Deployment
	err = r.Get(ctx, client.ObjectKey{Namespace: ds.Namespace, Name: depName}, &current)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	currApplyConfig, err := appsv1apply.ExtractDeployment(&current, "docserver-controller")
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(dep, currApplyConfig) {
		return nil
	}

	err = r.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "docserver-controller",
		Force:        pointer.Bool(true),
	})

	if err != nil {
		logger.Error(err, "unable to create or update Deployment")
		return err
	}
	logger.Info("reconcile Deployment successfully", "name", ds.Name)
	return nil
}

func (r *DocServerReconciler) reconcileService(ctx context.Context, ds updatev1beta1.DocServer) error {
	logger := log.FromContext(ctx)
	svcName := "docserver-" + ds.Name
	owner, err := controllerReference(ds, r.Scheme)
	if err != nil {
		return err
	}

	svc := corev1apply.Service(svcName, ds.Namespace).
		WithLabels(map[string]string{
			"app.kubernetes.io/name":       "mkdocs",
			"app.kubernetes.io/instance":   ds.Name,
			"app.kubernetes.io/created-by": "docserver-controller",
		}).
		WithOwnerReferences(owner).
		WithSpec(corev1apply.ServiceSpec().
			WithSelector(map[string]string{
				"app.kubernetes.io/name":       "mkdocs",
				"app.kubernetes.io/instance":   ds.Name,
				"app.kubernetes.io/created-by": "docserver-controller",
			}).
			WithType(corev1.ServiceTypeClusterIP).
			WithPorts(corev1apply.ServicePort().
				WithProtocol(corev1.ProtocolTCP).
				WithPort(8000).
				WithTargetPort(intstr.FromInt(8000)),
			),
		)

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(svc)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var current corev1.Service
	err = r.Get(ctx, client.ObjectKey{Namespace: ds.Namespace, Name: svcName}, &current)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	currApplyConfig, err := corev1apply.ExtractService(&current, "docserver-controller")
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(svc, currApplyConfig) {
		return nil
	}

	err = r.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "docserver-controller",
		Force:        pointer.Bool(true),
	})
	if err != nil {
		logger.Error(err, "unable to create or update Service")
		return err
	}

	logger.Info("reconcile Service successfully", "name", ds.Name)
	return nil
}

func (r *DocServerReconciler) updateStatus(ctx context.Context, ds updatev1beta1.DocServer) (ctrl.Result, error) {
	var dep appsv1.Deployment
	err := r.Get(ctx, client.ObjectKey{Namespace: ds.Namespace, Name: "docserver-" + ds.Name}, &dep)
	if err != nil {
		return ctrl.Result{}, err
	}

	var status updatev1beta1.DocServerStatus
	if dep.Status.AvailableReplicas == 0 {
		status = updatev1beta1.DocServerNotReady
	} else if dep.Status.AvailableReplicas == ds.Spec.Replicas {
		status = updatev1beta1.DocServerHealthy
	} else {
		status = updatev1beta1.DocServerAvailable
	}

	if ds.Status != status {
		ds.Status = status
		err = r.Status().Update(ctx, &ds)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if ds.Status != updatev1beta1.DocServerHealthy {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DocServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&updatev1beta1.DocServer{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1.Job{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func controllerReference(ds updatev1beta1.DocServer, scheme *runtime.Scheme) (*metav1apply.OwnerReferenceApplyConfiguration, error) {
	gvk, err := apiutil.GVKForObject(&ds, scheme)
	if err != nil {
		return nil, err
	}
	ref := metav1apply.OwnerReference().
		WithAPIVersion(gvk.GroupVersion().String()).
		WithKind(gvk.Kind).
		WithName(ds.Name).
		WithUID(ds.GetUID()).
		WithBlockOwnerDeletion(true).
		WithController(true)
	return ref, nil
}
