/*
Copyright 2021.

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

	elasticwebv1 "github.com/cloud-native/elasticweb/pkg/api/v1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	elasticFinalizer string = "elastic.finalizers.k8s.io"
)

// ElasticWebReconciler reconciles a ElasticWeb object
type ElasticWebReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=elasticweb.k8s.io,resources=elasticwebs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=elasticweb.k8s.io,resources=elasticwebs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=elasticweb.k8s.io,resources=elasticwebs/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ElasticWeb object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *ElasticWebReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//ctx = context.Background()
	log := r.Log.WithValues("elasticweb", req.NamespacedName)

	log.Info(fmt.Sprintf("1.start reconcile logic for %s", req.NamespacedName))
	instance := &elasticwebv1.ElasticWeb{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		log.Error(err, "cloud not fetch elasticwebs")
		return ctrl.Result{}, fmt.Errorf("cloud not fetch elasticwebs: %+v", err)
	}
	log.Info("2.get instance for:" + instance.String())
	// delete elasticweb instance
	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("3 enter delete elasticweb logic")
		if containsString(instance.ObjectMeta.Finalizers, elasticFinalizer) {
			instance.ObjectMeta.Finalizers = removeString(instance.ObjectMeta.Finalizers, elasticFinalizer)
		}
		if err := r.Update(ctx, instance); err != nil {
			log.Error(err, "3.1 update instance finalizer error for delete")
			return ctrl.Result{}, err
		}
		log.Info(fmt.Sprintf("3.2 delete elasticweb %s successful", req.NamespacedName))
		return ctrl.Result{}, nil
	}
	// create or update elasticweb instance
	log.Info(fmt.Sprintf("4 enter create or update elasticweb%s logic", req.NamespacedName))
	if !containsString(instance.ObjectMeta.Finalizers, elasticFinalizer) {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, elasticFinalizer)
		if err := r.Update(ctx, instance); err != nil {
			log.Error(err, fmt.Sprintf("4.1 update elasticweb%s finalizer error", req.NamespacedName))
			return ctrl.Result{}, err
		}
		log.Info(fmt.Sprintf("4.2 update elasticweb %s finalizer successful", req.NamespacedName))
	}
	log.Info(fmt.Sprintf("5 fetch deployment %s", req.NamespacedName))
	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, req.NamespacedName, deployment)
	if errors.IsNotFound(err) {
		// create elasticweb instance
		log.Info(fmt.Sprintf("5.1 deployment %s is not exists maybe need to create", req.NamespacedName))
		if *(instance.Spec.TotalQPS) < 1 {
			log.Info("5.2 not need deployment")
			return ctrl.Result{}, nil
		}
		if err = createServiceIfNotExists(ctx, r, instance, req); err != nil {
			log.Error(err, "5.3 create service error")
			return ctrl.Result{}, err
		}
		if err = createDeployment(ctx, r, instance); err != nil {
			log.Error(err, "5.4 create deployment error")
			return ctrl.Result{}, err
		}
		if err = updateStatus(ctx, r, instance); err != nil {
			log.Error(err, "5.5 update instance status error")
			return ctrl.Result{}, err
		}
		log.Info(fmt.Sprintf("5.6 create elasticweb instance %s successful", req.NamespacedName))
		return ctrl.Result{}, err
	}
	if err != nil {
		log.Error(err, "5.7 fetch deployment failed")
		return ctrl.Result{}, err
	}
	// update elasticweb instance, need to calc expect replicas and compare with the actual relicas,
	// then adjust to expect replicas
	log.Info(fmt.Sprintf("6 deployment %s is exist. "+
		"need to calc replicas and compare it with real deploy replica", req.NamespacedName))
	// calc expect replicas
	expectReplicas := getExpectReplicas(instance)
	// get actual replicas
	realReplicas := *deployment.Spec.Replicas
	log.Info(fmt.Sprintf("7 expectReplicas [%d], realReplicas [%d]", expectReplicas, realReplicas))
	if expectReplicas == realReplicas {
		log.Info("7.1 no need to update deployment replica, return now")
		return ctrl.Result{}, nil
	}
	// ajust actual replicas to expect and update it
	*(deployment.Spec.Replicas) = expectReplicas
	log.Info("8 update deployment's Replicas")
	if err = r.Update(ctx, deployment); err != nil {
		log.Error(err, "8.1 update deployment replicas error")
		return ctrl.Result{}, err
	}
	log.Info(fmt.Sprintf("9 update deployment %s status", req.NamespacedName))
	// update the elasticweb instance status
	if err = updateStatus(ctx, r, instance); err != nil {
		log.Error(err, fmt.Sprintf("9.1 update elasticweb %s status error", req.NamespacedName))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElasticWebReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&elasticwebv1.ElasticWeb{}).
		Complete(r)
}

// update elasticweb instance status
func updateStatus(ctx context.Context, r *ElasticWebReconciler, e *elasticwebv1.ElasticWeb) error {
	log := r.Log.WithValues("func", "updateStatus")
	// single pod qps
	singlePodQPS := *(e.Spec.SinglePodQPS)
	// total pod num
	replicas := getExpectReplicas(e)
	if e.Status.RealQPS == nil {
		e.Status.RealQPS = new(int32)
	}
	*(e.Status.RealQPS) = singlePodQPS * replicas
	log.Info(fmt.Sprintf("singlePodQPS [%d], replicas [%d], realQps[%d]",
		singlePodQPS, replicas, *(e.Status.RealQPS)))
	if err := r.Status().Update(ctx, e); err != nil {
		log.Error(err, fmt.Sprintf("update elasticweb %s/%s status failed", e.Namespace, e.Name))
		return err
	}
	return nil
}

//create service
func createServiceIfNotExists(ctx context.Context, r *ElasticWebReconciler,
	e *elasticwebv1.ElasticWeb, req ctrl.Request) error {
	log := r.Log.WithValues("func", "createService")
	service := &corev1.Service{}
	err := r.Get(ctx, req.NamespacedName, service)
	if err == nil {
		log.Info(fmt.Sprintf("service %s if exists, no need to create", req.NamespacedName))
		return nil
	}
	if !errors.IsNotFound(err) {
		log.Error(err, fmt.Sprintf("query service %s failed", req.NamespacedName))
		return err
	}
	service = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: e.Namespace,
			Name:      e.Name,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Port:     8080,
					NodePort: *e.Spec.Port,
				},
			},
			Selector: map[string]string{
				"app": AppName,
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}
	log.Info(fmt.Sprintf("set reference for elasticweb %s and service %s", e.Name, service.Name))
	if err := controllerutil.SetControllerReference(e, service, r.Scheme); err != nil {
		log.Error(err, fmt.Sprintf("set reference for elasticweb %s and service %s failed",
			e.Name, service.Name))
		return err
	}
	log.Info("start create service")
	if err := r.Create(ctx, service); err != nil {
		r.Recorder.Eventf(e, "Warning", "Create Service", "Create service %s/%s failed",
			service.Namespace, service.Name)
		log.Error(err, fmt.Sprintf("create service %s/%s failed", service.Namespace, service.Name))
		return err
	}
	r.Recorder.Eventf(e, "Normal", "Create Service",
		"Create service %s/%s successful", service.Namespace, service.Name)
	log.Info(fmt.Sprintf("create service %s/%s success", service.Namespace, service.Name))
	return nil
}

// crate deployment
func createDeployment(ctx context.Context, r *ElasticWebReconciler, e *elasticwebv1.ElasticWeb) error {
	log := r.Log.WithValues("func", "createDeployment")
	expectReplicas := getExpectReplicas(e)
	log.Info(fmt.Sprintf("expectReplicas [%d]", expectReplicas))
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: e.Namespace,
			Name:      e.Name,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(expectReplicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": AppName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": AppName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            AppName,
						Image:           e.Spec.Image,
						ImagePullPolicy: "IfNotPresent",
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								Protocol:      corev1.ProtocolSCTP,
								ContainerPort: ContainerPort,
							},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu":    resource.MustParse(CPULimit),
								"memory": resource.MustParse(MemLimit),
							},
							Requests: corev1.ResourceList{
								"cpu":    resource.MustParse(CPURequest),
								"memory": resource.MustParse(MemRequest),
							},
						},
					},
					},
				},
			},
		},
	}
	log.Info(fmt.Sprintf("set refrence for elasticweb %s and deployment %s", e.Name, deployment.Name))
	if err := controllerutil.SetOwnerReference(e, deployment, r.Scheme); err != nil {
		log.Error(err, fmt.Sprintf("set reference for elasticweb %s and deployment %s failed",
			e.Name, deployment.Name))
		return err
	}
	log.Info("start create deployment")
	if err := r.Create(ctx, deployment); err != nil {
		r.Recorder.Eventf(e,
			"Normal", "Create Deployment", "Create deployment %s successful", e.Name)
		log.Info(fmt.Sprintf("create deployment %s/%s success", deployment.Namespace, deployment.Name))
	}
	return nil
}
