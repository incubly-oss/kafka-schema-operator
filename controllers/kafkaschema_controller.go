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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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
	logger := log.FromContext(ctx)

	schema := &kafkaschemaoperatorv2beta1.KafkaSchema{}
	err := r.Get(ctx, req.NamespacedName, schema)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Schema resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Schema resource")
		return ctrl.Result{}, err
	}

	var maybePassword string
	secretName := schema.SchemaRegistry.PasswordSecret
	if len(secretName) > 0 {
		secret := &v1.Secret{}
		err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: schema.Namespace}, secret)
		if err != nil {
			logger.Error(err, "Failed to get Secret with schema registry password "+secretName)
			return ctrl.Result{}, err
		}
		maybePassword = string(secret.Data[schema.SchemaRegistry.SecretKey])
	}

	srClient, err := schemareg.NewClient(&schema.SchemaRegistry, maybePassword, logger)
	if err != nil {
		logger.Error(err, "Failed to instantiate Schema Registry Client")
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
				logger.Error(err, "Failed to delete KafkaSchema from kubernetes: "+schema.Name)
				return ctrl.Result{}, err
			}
			logger.Info("KafkaSchema CR was deleted: " + schema.Name)

			err := srClient.DeleteSubject(schemaKey)
			if err != nil {
				logger.Error(err, "Failed to delete subject for key schema from registry: "+schemaKey)
				return ctrl.Result{}, err
			}

			err = srClient.DeleteSubject(schemaValue)
			if err != nil {
				logger.Error(err, "Failed to delete subject for value schema from registry: "+schemaValue)
				return ctrl.Result{}, err
			}
			logger.Info("Schema was removed from registry")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(schema, schemaFinilizers) {
		controllerutil.AddFinalizer(schema, schemaFinilizers)
		err = r.Update(ctx, schema)
		if err != nil {
			logger.Info("Failed to update finalizers for CR: " + schema.Name)
			return ctrl.Result{}, err
		}
		logger.Info("Finalizers are set for CR: " + schema.Name)
	}

	err = srClient.RegisterSchema(
		schemaKey,
		schemareg.RegisterSchemaReq{
			Schema: `{"type": "` + schema.Spec.SchemaSerializer + `"}`,
		})
	if err != nil {
		logger.Error(err, "Failed to update schema registry")
		return reconcileResult, err
	}
	logger.Info("Schema key was published: " + schemaKey)

	err = srClient.RegisterSchema(
		schemaValue,
		schemareg.RegisterSchemaReq{
			Schema:     schema.Spec.Data.Schema,
			SchemaType: strings.ToUpper(schema.Spec.Data.Format),
		})
	if err != nil {
		logger.Error(err, "Failed to update schema registry")
		return reconcileResult, err
	}

	err = srClient.SetCompatibilityMode(
		schemaValue,
		schemareg.SetCompatibilityModeReq{
			Compatibility: schema.Spec.Data.Compatibility,
		},
	)

	if err != nil {
		logger.Error(err, "Failed to update schema compatibility for value")
		return reconcileResult, err
	}

	err = srClient.SetCompatibilityMode(
		schemaKey,
		schemareg.SetCompatibilityModeReq{
			Compatibility: schema.Spec.Data.Compatibility,
		},
	)

	if err != nil {
		logger.Error(err, "Failed to update schema compatibility for key")
		return reconcileResult, err
	}

	logger.Info("Schema value was published: " + schemaValue)
	return reconcileResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KafkaSchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafkaschemaoperatorv2beta1.KafkaSchema{}).
		Complete(r)
}
