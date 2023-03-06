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
	er "errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kafkaschemaoperatorv1beta1 "kafka-schema-operator/api/v1beta1"
)

// KafkaSchemaReconciler reconciles a KafkaSchema object
type KafkaSchemaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func generateSchemaUrl(subject string) (string, error) {
	schemaRegistryHost := os.Getenv("SCHEMA_REGISTRY_HOST")
	schemaRegistryPort := os.Getenv("SCHEMA_REGISTRY_PORT")
	if len(schemaRegistryHost) < 0 || len(schemaRegistryPort) < 0 {
		return "", er.New("Schema registry or port is not set")
	}
	var url strings.Builder
	url.WriteString("http://")
	url.WriteString(schemaRegistryHost)
	url.WriteString(":")
	url.WriteString(schemaRegistryPort)
	url.WriteString("/subjects/")
	url.WriteString(subject)
	url.WriteString("/versions")
	return url.String(), nil
}

func registerNewSchema(ctx context.Context, url string, payload string) error {
	log := log.FromContext(ctx)
	httpReq, _ := http.NewRequest("POST", url, strings.NewReader(payload))
	httpReq.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	if len(os.Getenv("SCHEMA_REGISTRY_KEY")) > 0 || len(os.Getenv("SCHEMA_REGISTRY_SECRET")) > 0 {
		httpReq.SetBasicAuth(os.Getenv("SCHEMA_REGISTRY_KEY"), os.Getenv("SCHEMA_REGISTRY_SECRET"))
	}
	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Error(err, "Failed to send schema payload to schema-registry")
		return err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(httpResp.Body)
		if err != nil {
			log.Error(err, "Cannot read http response body")
		}
		log.Info("Failed to update schema registry")
		log.Info("Statuscode: " + strconv.Itoa(httpResp.StatusCode))
		log.Info("Body response: " + string(bodyBytes))
		return er.New("Failed to update schema registry: " + string(bodyBytes))
	}
	return nil
}

func (r *KafkaSchemaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	schema := &kafkaschemaoperatorv1beta1.KafkaSchema{}
	err := r.Get(ctx, req.NamespacedName, schema)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Schema resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Schema resource")
		return ctrl.Result{Requeue: true}, err
	}

	cfg := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: schema.Spec.Data.ConfigRef, Namespace: schema.Namespace}, cfg)
	if err != nil {
		log.Error(err, "Failed to find ConfigMap: "+schema.Spec.Data.ConfigRef)
		return ctrl.Result{Requeue: true}, err
	}

	schemaKey := schema.Spec.Name + "-key"
	schemaValue := schema.Spec.Name + "-value"

	keySchemaRegistryUrl, err := generateSchemaUrl(schemaKey)
	if err != nil {
		log.Error(err, "Cannot create registry url")
		return ctrl.Result{Requeue: true}, err
	}

	valueSchemaRegistryUrl, err := generateSchemaUrl(schemaValue)
	if err != nil {
		log.Error(err, "Cannot create registry url")
		return ctrl.Result{Requeue: true}, err
	}

	var schemaKeyPayload strings.Builder
	schemaKeyPayload.WriteString(`{"schema": "{\"type\": \"`)
	schemaKeyPayload.WriteString(schema.Spec.SchemaSerializer)
	schemaKeyPayload.WriteString(`\"}"}`)

	err = registerNewSchema(ctx, valueSchemaRegistryUrl, schemaKeyPayload.String())
	if err != nil {
		log.Error(err, "Failed to update schema registry")
		return ctrl.Result{Requeue: true}, err
	}
	log.Info("Schema key was published: " + schemaKey)

	cfgData := cfg.Data["schema"]
	cfgData = strings.ReplaceAll(cfgData, "\n", "")
	cfgData = strings.ReplaceAll(cfgData, "\t", "")
	cfgData = strings.ReplaceAll(cfgData, " ", "")
	cfgData = strings.ReplaceAll(cfgData, `"`, `\"`)
	cfgData = strings.Replace(cfgData, `\"{`, `"{`, 1)
	cfgData = strings.Replace(cfgData, `}\"`, `}"`, -1)

	var schemaValuePayload strings.Builder
	schemaValuePayload.WriteString(`{"schema": "`)
	schemaValuePayload.WriteString(cfgData)
	schemaValuePayload.WriteString(`",`)
	schemaValuePayload.WriteString(`"schemaType": "`)
	schemaValuePayload.WriteString(strings.ToUpper(schema.Spec.Data.Format))
	schemaValuePayload.WriteString(`"}`)

	err = registerNewSchema(ctx, keySchemaRegistryUrl, schemaValuePayload.String())
	if err != nil {
		log.Error(err, "Failed to update schema registry")
		return ctrl.Result{Requeue: true}, err
	}

	log.Info("Schema value was published: " + schemaValue)
	return ctrl.Result{Requeue: true}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KafkaSchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafkaschemaoperatorv1beta1.KafkaSchema{}).
		Complete(r)
}
