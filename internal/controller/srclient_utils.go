package controller

import (
	"context"
	"fmt"
	"github.com/riferrei/srclient"
	"incubly.oss/kafka-schema-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/url"
	"os"
)

func extractSchemaRegistryUrl(registry v1beta1.SchemaRegistry) (*url.URL, error) {
	if len(registry.BaseUrl) > 0 {
		return url.Parse(registry.BaseUrl)
	}
	defaultBaseUrl := os.Getenv("SCHEMA_REGISTRY_BASE_URL")
	if len(defaultBaseUrl) > 0 {
		return url.Parse(defaultBaseUrl)
	} else {
		return nil, fmt.Errorf("base URL for schema registry not set")
	}
}

func (r *KafkaSchemaReconciler) SetCredentials(
	ctx context.Context, srClient *srclient.SchemaRegistryClient, req *v1beta1.KafkaSchema) error {
	basicAuth := req.Spec.SchemaRegistry.BasicAuth
	if len(basicAuth.Username) > 0 {
		password, err := r.readPasswordFromSecret(ctx, req, basicAuth)
		if err != nil {
			return err
		}
		srClient.SetCredentials(basicAuth.Username, password)
		return nil
	}
	maybeUser := os.Getenv("SCHEMA_REGISTRY_BASIC_AUTH_USERNAME")
	maybePassword := os.Getenv("SCHEMA_REGISTRY_BASIC_AUTH_PASSWORD")
	if len(maybeUser) > 0 && len(maybePassword) > 0 {
		srClient.SetCredentials(maybeUser, maybePassword)
		return nil
	}
	// implement other auth methods (e.g. OAuth?) if needed
	return nil
}

func (r *KafkaSchemaReconciler) readPasswordFromSecret(
	ctx context.Context, req *v1beta1.KafkaSchema, basicAuth v1beta1.BasicAuth) (string, error) {
	secret := &v1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: req.Namespace,
		Name:      basicAuth.PasswordSecretName,
	}, secret)
	if err != nil {
		return "", err
	}
	passwordBarr := secret.Data[basicAuth.PasswordSecretKey]
	if len(passwordBarr) == 0 {
		return "", fmt.Errorf(
			"key %s not found in secret %s",
			basicAuth.PasswordSecretKey,
			basicAuth.PasswordSecretName)
	}
	password := string(passwordBarr)
	return password, nil
}
