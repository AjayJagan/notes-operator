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
	"reflect"

	notesv1alpha1 "github.com/AjayJagan/notes-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NotesAppReconciler reconciles a NotesApp object
type NotesAppReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=notes.notes.com,resources=notesapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notes.notes.com,resources=notesapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=notes.notes.com,resources=notesapps/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events;services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=pods;ingresses,verbs=get;list;watch;create;update;patch;delete
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NotesApp object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile

func (r *NotesAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// TODO(user): your logic here
	instance := &notesv1alpha1.NotesApp{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("notesapp resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get notesapp")
		return ctrl.Result{}, err
	}
	err = r.NotesApp(instance, log)
	if err != nil {
		return ctrl.Result{}, err
	}
	_ = r.Client.Status().Update(context.TODO(), instance)
	return ctrl.Result{}, nil
}

func (r *NotesAppReconciler) NotesApp(notesapp *notesv1alpha1.NotesApp, log logr.Logger) error {
	// deploy ingress
	ingress := getIngressService(notesapp)
	naIngress := &networkingv1.Ingress{}
	ingressErr := r.Client.Get(context.TODO(), types.NamespacedName{Name: "ingress-service", Namespace: notesapp.Namespace}, naIngress)
	if ingressErr != nil {
		if apierrors.IsNotFound(ingressErr) {
			controllerutil.SetControllerReference(notesapp, ingress, r.Scheme)
			ingressErr := r.Client.Create(context.TODO(), ingress)
			if ingressErr != nil {
				return ingressErr
			}
		} else {
			log.Info("failed to get the ingress service")
			return ingressErr
		}
	} else if !reflect.DeepEqual(ingress.Spec, naIngress.Spec) {
		ingress.ObjectMeta = naIngress.ObjectMeta
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number = notesapp.Spec.NginxIngress.FePort
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[1].Backend.Service.Port.Number = notesapp.Spec.NginxIngress.BePort
		controllerutil.SetControllerReference(notesapp, ingress, r.Scheme)
		ingressErr = r.Client.Update(context.TODO(), ingress)
		if ingressErr != nil {
			return ingressErr
		}
		log.Info("notes app ingress config updated")
	}

	// mongo persistant volume claim
	mongoPvc := getMongoPvc(notesapp)
	mpvc := &corev1.PersistentVolumeClaim{}
	mpvcErr := r.Client.Get(context.TODO(), types.NamespacedName{Name: "mongo-persistent-volume-claim", Namespace: notesapp.Namespace}, mpvc)
	if mpvcErr != nil {
		if apierrors.IsNotFound(mpvcErr) {
			controllerutil.SetControllerReference(notesapp, mongoPvc, r.Scheme)
			mpvcErr = r.Client.Create(context.TODO(), mongoPvc)
			if mpvcErr != nil {
				return mpvcErr
			}
			log.Info("mongo pvc created")
		} else {
			log.Info("failed to get mongo pvc")
			return mpvcErr
		}
	}
	// mongo service
	mongoSvc := getMongoService(notesapp)
	msvc := &corev1.Service{}
	msvcErr := r.Client.Get(context.TODO(), types.NamespacedName{Name: "mongo-clusterip-service", Namespace: notesapp.Namespace}, msvc)
	if msvcErr != nil {
		if apierrors.IsNotFound(msvcErr) {
			controllerutil.SetControllerReference(notesapp, mongoSvc, r.Scheme)
			msvcErr = r.Client.Create(context.TODO(), mongoSvc)
			if msvcErr != nil {
				return msvcErr
			}
		} else {
			log.Info("failed to get mongo service")
			return msvcErr
		}
	} else if !reflect.DeepEqual(mongoSvc.Spec, msvc.Spec) {
		mongoSvc.ObjectMeta = msvc.ObjectMeta
		mongoSvc.Spec.Ports[0].Port = notesapp.Spec.MongoDb.Port
		mongoSvc.Spec.Ports[0].TargetPort = intstr.FromInt(notesapp.Spec.MongoDb.TargetPort)
		controllerutil.SetControllerReference(notesapp, mongoSvc, r.Scheme)
		msvcErr = r.Client.Update(context.TODO(), mongoSvc)
		if msvcErr != nil {
			return msvcErr
		}
		log.Info("mongo service updated")
	}

	// mongo deployment
	mongoDep := getMongoDeployment(notesapp)
	mongod := &appsv1.Deployment{}
	mongodErr := r.Client.Get(context.TODO(), types.NamespacedName{Name: "mongodb-deployment", Namespace: notesapp.Namespace}, mongod)
	if mongodErr != nil {
		if apierrors.IsNotFound(mongodErr) {
			controllerutil.SetControllerReference(notesapp, mongoDep, r.Scheme)
			mongodErr = r.Client.Create(context.TODO(), mongoDep)
			if mongodErr != nil {
				return mongodErr
			}
		} else {
			log.Info("failed to get mongo deployment")
			return mongodErr
		}
	} else if !reflect.DeepEqual(mongoDep.Spec, mongod.Spec) {
		mongoDep.ObjectMeta = mongod.ObjectMeta
		mongoDep.Spec.Replicas = &notesapp.Spec.MongoDb.Replicas
		mongoDep.Spec.Template.Spec.Containers[0].Image = notesapp.Spec.MongoDb.Repository + ":" + notesapp.Spec.MongoDb.Tag
		mongoDep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = notesapp.Spec.MongoDb.Port
		controllerutil.SetControllerReference(notesapp, mongoDep, r.Scheme)
		mongodErr = r.Client.Update(context.TODO(), mongoDep)
		if mongodErr != nil {
			return mongodErr
		}
		log.Info("mongo deployment updated")
	}

	//backend serivce
	backendSvc := getBackendService(notesapp)
	beSvc := &corev1.Service{}
	beSvcErr := r.Client.Get(context.TODO(), types.NamespacedName{Name: "notesapp-be-clusterip-service", Namespace: notesapp.Namespace}, beSvc)
	if beSvcErr != nil {
		if apierrors.IsNotFound(beSvcErr) {
			controllerutil.SetControllerReference(notesapp, backendSvc, r.Scheme)
			beSvcErr = r.Client.Create(context.TODO(), backendSvc)
			if beSvcErr != nil {
				return beSvcErr
			}
		} else {
			log.Info("failed to get notesapp backend service")
			return beSvcErr
		}
	} else if !reflect.DeepEqual(backendSvc.Spec, beSvc.Spec) {
		backendSvc.ObjectMeta = beSvc.ObjectMeta
		backendSvc.Spec.Ports[0].Port = notesapp.Spec.NotesAppBe.Port
		backendSvc.Spec.Ports[0].TargetPort = intstr.FromInt(notesapp.Spec.NotesAppBe.TargetPort)
		controllerutil.SetControllerReference(notesapp, backendSvc, r.Scheme)
		beSvcErr = r.Client.Update(context.TODO(), backendSvc)
		if beSvcErr != nil {
			return beSvcErr
		}
		log.Info("notesapp backend service updated")
	}
	// backend deployment
	backendDep := getBackendDeployment(notesapp)
	beDep := &appsv1.Deployment{}
	beDepErr := r.Client.Get(context.TODO(), types.NamespacedName{Name: "notesapp-be-deployment", Namespace: notesapp.Namespace}, beDep)
	if beDepErr != nil {
		if apierrors.IsNotFound(beDepErr) {
			controllerutil.SetControllerReference(notesapp, backendDep, r.Scheme)
			beDepErr = r.Client.Create(context.TODO(), backendDep)
			if beDepErr != nil {
				return beDepErr
			}
		} else {
			log.Info("failed to get notesapp backend deployment")
			return beDepErr
		}
	} else if !reflect.DeepEqual(backendDep.Spec, beDep.Spec) {
		backendDep.ObjectMeta = beDep.ObjectMeta
		backendDep.Spec.Replicas = &notesapp.Spec.NotesAppBe.Replicas
		backendDep.Spec.Template.Spec.Containers[0].Image = notesapp.Spec.NotesAppBe.Repository + ":" + notesapp.Spec.NotesAppBe.Tag
		backendDep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = notesapp.Spec.NotesAppBe.Port
		controllerutil.SetControllerReference(notesapp, backendDep, r.Scheme)
		beDepErr = r.Client.Update(context.TODO(), backendDep)
		if beDepErr != nil {
			return beDepErr
		}
		log.Info("notesapp backend deployment updated")
	}
	//frontend service
	frontendSvc := getFrontendService(notesapp)
	feSvc := &corev1.Service{}
	feSvcErr := r.Client.Get(context.TODO(), types.NamespacedName{Name: "notesapp-fe-clusterip-service", Namespace: notesapp.Namespace}, feSvc)
	if feSvcErr != nil {
		if apierrors.IsNotFound(feSvcErr) {
			controllerutil.SetControllerReference(notesapp, frontendSvc, r.Scheme)
			feSvcErr = r.Client.Create(context.TODO(), frontendSvc)
			if feSvcErr != nil {
				return feSvcErr
			}
		} else {
			log.Info("failed to get notesapp frontend service")
			return feSvcErr
		}
	} else if !reflect.DeepEqual(frontendSvc.Spec, feSvc.Spec) {
		frontendSvc.ObjectMeta = feSvc.ObjectMeta
		frontendSvc.Spec.Ports[0].Port = notesapp.Spec.NotesAppFe.Port
		frontendSvc.Spec.Ports[0].TargetPort = intstr.FromInt(notesapp.Spec.NotesAppFe.TargetPort)
		controllerutil.SetControllerReference(notesapp, frontendSvc, r.Scheme)
		feSvcErr = r.Client.Update(context.TODO(), frontendSvc)
		if feSvcErr != nil {
			return feSvcErr
		}
		log.Info("notesapp frontend service updated")
	}
	//frontend deployment
	frontendDep := getFrontendDeployment(notesapp)
	feDep := &appsv1.Deployment{}
	feDepErr := r.Client.Get(context.TODO(), types.NamespacedName{Name: "notesapp-fe-deployment", Namespace: notesapp.Namespace}, feDep)
	if feDepErr != nil {
		if apierrors.IsNotFound(feDepErr) {
			controllerutil.SetControllerReference(notesapp, frontendDep, r.Scheme)
			feDepErr = r.Client.Create(context.TODO(), frontendDep)
			if feDepErr != nil {
				return feDepErr
			}
		} else {
			log.Info("failed to get notesapp frontend deployment")
			return feDepErr
		}
	} else if !reflect.DeepEqual(frontendDep.Spec, feDep.Spec) {
		frontendDep.ObjectMeta = feDep.ObjectMeta
		frontendDep.Spec.Replicas = &notesapp.Spec.NotesAppFe.Replicas
		frontendDep.Spec.Template.Spec.Containers[0].Image = notesapp.Spec.NotesAppFe.Repository + ":" + notesapp.Spec.NotesAppFe.Tag
		frontendDep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = notesapp.Spec.NotesAppFe.Port
		controllerutil.SetControllerReference(notesapp, frontendDep, r.Scheme)
		feDepErr = r.Client.Update(context.TODO(), frontendDep)
		if feDepErr != nil {
			return feDepErr
		}
		log.Info("notesapp front deployment updated")
	}
	return nil
}

func getIngressService(notesapp *notesv1alpha1.NotesApp) *networkingv1.Ingress {
	pt := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ingress-service",
			Namespace: notesapp.Namespace,
			Labels:    map[string]string{"app": "notesapp-ingress-service"},
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":                "nginx",
				"nginx.ingress.kubernetes.io/use-regex":      "true",
				"nginx.ingress.kubernetes.io/rewrite-target": "/$1",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/?(.*)",
									PathType: &pt,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "notesapp-fe-clusterip-service",
											Port: networkingv1.ServiceBackendPort{
												Number: notesapp.Spec.NginxIngress.FePort,
											},
										},
									},
								},
								{
									Path:     "/api/?(.*)",
									PathType: &pt,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "notesapp-be-clusterip-service",
											Port: networkingv1.ServiceBackendPort{
												Number: notesapp.Spec.NginxIngress.BePort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return ing
}

func getMongoService(notesapp *notesv1alpha1.NotesApp) *corev1.Service {
	p := make([]corev1.ServicePort, 0)
	servicePort := corev1.ServicePort{
		Name:       "tcp-port",
		Port:       notesapp.Spec.MongoDb.Port,
		TargetPort: intstr.FromInt(notesapp.Spec.MongoDb.TargetPort),
	}
	p = append(p, servicePort)
	mongoSvc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mongo-clusterip-service",
			Namespace: notesapp.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"component": "mongo"},
			Ports:    p,
		},
	}
	return mongoSvc
}

func getMongoPvc(notesapp *notesv1alpha1.NotesApp) *corev1.PersistentVolumeClaim {
	mongoPvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mongo-persistent-volume-claim",
			Namespace: notesapp.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("2Gi"),
				},
			},
		},
	}
	return mongoPvc
}
func getMongoDeployment(notesapp *notesv1alpha1.NotesApp) *appsv1.Deployment {
	mongoDep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mongodb-deployment",
			Namespace: notesapp.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &notesapp.Spec.MongoDb.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"component": "mongo"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"component": "mongo"},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "mongo-storage",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "mongo-persistent-volume-claim",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "mongo",
							Image: notesapp.Spec.MongoDb.Repository + ":" + notesapp.Spec.MongoDb.Tag,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: notesapp.Spec.MongoDb.Port,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "mongo-storage",
									MountPath: "/data/db",
								},
							},
						},
					},
				},
			},
		},
	}
	return mongoDep
}

func getBackendService(notesapp *notesv1alpha1.NotesApp) *corev1.Service {
	p := make([]corev1.ServicePort, 0)
	servicePort := corev1.ServicePort{
		Name:       "tcp-port",
		Port:       notesapp.Spec.NotesAppBe.Port,
		TargetPort: intstr.FromInt(notesapp.Spec.NotesAppBe.TargetPort),
	}
	p = append(p, servicePort)
	beSvc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "notesapp-be-clusterip-service",
			Namespace: notesapp.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"component": "notesapp-be"},
			Ports:    p,
		},
	}
	return beSvc
}

func getBackendDeployment(notesapp *notesv1alpha1.NotesApp) *appsv1.Deployment {
	beDep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "notesapp-be-deployment",
			Namespace: notesapp.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &notesapp.Spec.NotesAppBe.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"component": "notesapp-be"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"component": "notesapp-be"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "notesapp-be",
							Image: notesapp.Spec.NotesAppBe.Repository + ":" + notesapp.Spec.NotesAppBe.Tag,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: notesapp.Spec.NotesAppBe.Port,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "MONGO_HOST",
									Value: "mongo-clusterip-service",
								},
								{
									Name:  "MONGO_PORT",
									Value: "27017",
								},
								{
									Name:  "SERVER_PORT",
									Value: "3000",
								},
								{
									Name:  "MONGO_DB_NAME",
									Value: "NoteDB",
								},
							},
						},
					},
				},
			},
		},
	}
	return beDep
}

func getFrontendService(notesapp *notesv1alpha1.NotesApp) *corev1.Service {
	p := make([]corev1.ServicePort, 0)
	servicePort := corev1.ServicePort{
		Name:       "tcp-port",
		Port:       notesapp.Spec.NotesAppFe.Port,
		TargetPort: intstr.FromInt(notesapp.Spec.NotesAppFe.TargetPort),
	}
	p = append(p, servicePort)
	beSvc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "notesapp-fe-clusterip-service",
			Namespace: notesapp.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"component": "notesapp-fe"},
			Ports:    p,
		},
	}
	return beSvc
}

func getFrontendDeployment(notesapp *notesv1alpha1.NotesApp) *appsv1.Deployment {
	beDep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "notesapp-fe-deployment",
			Namespace: notesapp.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &notesapp.Spec.NotesAppFe.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"component": "notesapp-fe"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"component": "notesapp-fe"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "notesapp-fe",
							Image: notesapp.Spec.NotesAppFe.Repository + ":" + notesapp.Spec.NotesAppFe.Tag,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: notesapp.Spec.NotesAppFe.Port,
								},
							},
						},
					},
				},
			},
		},
	}
	return beDep
}

// SetupWithManager sets up the controller with the Manager.
func (r *NotesAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&notesv1alpha1.NotesApp{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
