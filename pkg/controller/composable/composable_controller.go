/*

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

package composable

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"

	ibmcloudv1alpha1 "github.ibm.com/seed/composable/pkg/apis/ibmcloud/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Composable Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this ibmcloud.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileComposable{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("composable-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Composable
	err = c.Watch(&source.Kind{Type: &ibmcloudv1alpha1.Composable{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODOMV: Replace here with type created
	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by Composable - change this for objects you create
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &ibmcloudv1alpha1.Composable{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileComposable{}

// ReconcileComposable reconciles a Composable object
type ReconcileComposable struct {
	client.Client
	scheme *runtime.Scheme
}

func toJSONFromRaw(content *runtime.RawExtension) (interface{}, error) {
	var data interface{}

	if err := json.Unmarshal(content.Raw, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func resolve(object interface{}) unstructured.Unstructured {
	//TODO: if namespace is undefined, then define it
	obj := object.(map[string]interface{})
	ret := unstructured.Unstructured{
		Object: obj,
	}
	return ret
}

func getName(obj map[string]interface{}) (string, error) {
	metadata := obj["metadata"].(map[string]interface{})
	if name, ok := metadata["name"]; ok {
		return name.(string), nil
	}
	return "", fmt.Errorf("Template does not contain name")
}

func getNamespace(obj map[string]interface{}) (string, error) {
	metadata := obj["metadata"].(map[string]interface{})
	if namespace, ok := metadata["namespace"]; ok {
		return namespace.(string), nil
	}
	return "", fmt.Errorf("Template does not contain namespace")
}

// Reconcile reads that state of the cluster for a Composable object and makes changes based on the state read
// and what is in the Composable.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ibmcloud.ibm.com,resources=composables,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileComposable) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Composable instance
	instance := &ibmcloudv1alpha1.Composable{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	object, err := toJSONFromRaw(instance.Spec.Template)
	if err != nil {
		return reconcile.Result{}, err
	}

	resource := resolve(object)
	log.Println(resource)

	name, err := getName(resource.Object)
	if err != nil {
		log.Printf(err.Error())
		return reconcile.Result{}, err
	}

	log.Println("Resource name is: " + name)

	namespace, err := getNamespace(resource.Object)
	if err != nil {
		log.Printf(err.Error())
		return reconcile.Result{}, err
	}

	log.Println("Resource namespace is: " + namespace)

	if err := controllerutil.SetControllerReference(instance, &resource, r.scheme); err != nil {
		log.Println(err.Error())
		return reconcile.Result{}, err
	}

	found := &unstructured.Unstructured{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, found)
	if err != nil && strings.Contains(err.Error(), "no matches") {
		log.Printf("Creating resource %s/%s\n", namespace, name)
		err = r.Create(context.TODO(), &resource)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		log.Println("*****" + err.Error())
		return reconcile.Result{}, err
	}

	// Update the found object and write the result back if there are any changes
	if !reflect.DeepEqual(resource.Object["Spec"], found.Object["Spec"]) {
		found.Object["Spec"] = resource.Object["Spec"]
		log.Printf("Updating Deployment %s/%s\n", namespace, name)
		err = r.Update(context.TODO(), found)
		if err != nil {
			log.Println(err.Error())
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}