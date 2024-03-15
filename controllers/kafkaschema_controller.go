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

package controllers

import (
	"context"
	"kafka-schema-operator/schemareg"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kafkaschemaoperatorv2beta1 "kafka-schema-operator/api/v2beta1"
)

const schemaFinilizers = "kafka-schema-operator.incubly/finalizer"

// KafkaSchemaReconciler reconciles a KafkaSchema object
type KafkaSchemaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *KafkaSchemaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	schema := &kafkaschemaoperatorv2beta1.KafkaSchema{}
	err := r.Get(ctx, req.NamespacedName, schema)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Schema resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Schema resource")
		return ctrl.Result{}, err
	}

	srClient, err := schemareg.NewClient(&schema.SchemaRegistry, log)
	if err != nil {
		log.Error(err, "Failed to instantiate Schema Registry Client")
		return ctrl.Result{}, err
	}

	reconcileResult := ctrl.Result{}
	if schema.Spec.AutoReconciliation {
		reconcileResult = ctrl.Result{Requeue: true}
	} else {
		reconcileResult = ctrl.Result{}
	}

	schemaKey := schema.Spec.Name + "-key"
	schemaValue := schema.Spec.Name + "-value"

	isKafkaSchemaMarkedDeleted := schema.GetDeletionTimestamp() != nil
	if isKafkaSchemaMarkedDeleted {
		if !schema.Spec.TerminationProtection {
			controllerutil.RemoveFinalizer(schema, schemaFinilizers)
			err = r.Update(ctx, schema)
			if err != nil {
				log.Error(err, "Failed to delete KafkaSchema from kubernetes: "+schema.Name)
				return ctrl.Result{}, err
			}
			log.Info("KafkaSchema CR was deleted: " + schema.Name)

			err := srClient.DeleteSubject(schemaKey)
			if err != nil {
				log.Error(err, "Failed to delete subject for key schema from registry: "+schemaKey)
				return ctrl.Result{}, err
			}

			err = srClient.DeleteSubject(schemaValue)
			if err != nil {
				log.Error(err, "Failed to delete subject for value schema from registry: "+schemaValue)
				return ctrl.Result{}, err
			}
			log.Info("Schema was removed from registry")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(schema, schemaFinilizers) {
		controllerutil.AddFinalizer(schema, schemaFinilizers)
		err = r.Update(ctx, schema)
		if err != nil {
			log.Info("Failed to update finalizers for CR: " + schema.Name)
			return ctrl.Result{}, err
		}
		log.Info("Finalizers are set for CR: " + schema.Name)
	}

	err = srClient.RegisterSchema(
		schemaKey,
		schemareg.RegisterSchemaReq{
			Schema: `{\"type\": \"` + schema.Spec.SchemaSerializer + `\"}`,
		})
	if err != nil {
		log.Error(err, "Failed to update schema registry")
		return reconcileResult, err
	}
	log.Info("Schema key was published: " + schemaKey)

	cfgData := schema.Spec.Data.Schema
	cfgData = strings.ReplaceAll(cfgData, "\n", "")
	cfgData = strings.ReplaceAll(cfgData, "\t", "")
	cfgData = strings.ReplaceAll(cfgData, " ", "")
	cfgData = strings.ReplaceAll(cfgData, `"`, `\"`)
	cfgData = strings.Replace(cfgData, `\"{`, `"{`, 1)
	cfgData = strings.Replace(cfgData, `}\"`, `}"`, -1)

	err = srClient.RegisterSchema(
		schemaValue,
		schemareg.RegisterSchemaReq{
			Schema:     cfgData,
			SchemaType: strings.ToUpper(schema.Spec.Data.Format),
		})
	if err != nil {
		log.Error(err, "Failed to update schema registry")
		return reconcileResult, err
	}

	var schemaCompatibilityPayload strings.Builder
	schemaCompatibilityPayload.WriteString(`{"compatibility": "`)
	schemaCompatibilityPayload.WriteString(schema.Spec.Data.Compatibility)
	schemaCompatibilityPayload.WriteString(`"}`)

	err = srClient.SetCompatibilityMode(schemaValue, schemaCompatibilityPayload.String())
	if err != nil {
		log.Error(err, "Failed to update schema compatibility for value")
		return reconcileResult, err
	}

	err = srClient.SetCompatibilityMode(schemaKey, schemaCompatibilityPayload.String())
	if err != nil {
		log.Error(err, "Failed to update schema compatibility for key")
		return reconcileResult, err
	}

	log.Info("Schema value was published: " + schemaValue)
	return reconcileResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KafkaSchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafkaschemaoperatorv2beta1.KafkaSchema{}).
		Complete(r)
}
