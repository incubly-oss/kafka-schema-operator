package controllers

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"kafka-schema-operator/api/v2beta1"
)

const ResourceNs = "default"

func aKafkaSchemaResource(topicName string) *v2beta1.KafkaSchema {
	return &v2beta1.KafkaSchema{
		TypeMeta: v1.TypeMeta{
			Kind:       "KafkaSchema",
			APIVersion: "kafka-schema-operator.incubly/v2beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      topicName,
			Namespace: ResourceNs,
		},
		Spec: v2beta1.KafkaSchemaSpec{
			Name:                  topicName,
			SchemaSerializer:      "string",
			AutoReconciliation:    false,
			TerminationProtection: false,
			Data: v2beta1.KafkaSchemaData{
				Schema: `{
						  "namespace": "testing",
						  "type": "record",
						  "name": "testing",
						  "fields": [
						    {"name": "id", "type": "string"},
						    {"name": "email", "type": "string"}
						  ]
						}`,
				Format:        "AVRO",
				Compatibility: "BACKWARD",
			},
		},
	}
}

func resourceLookupKey(topicName string) types.NamespacedName {
	return types.NamespacedName{Name: topicName, Namespace: ResourceNs}
}
