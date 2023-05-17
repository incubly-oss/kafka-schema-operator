# Kafka Schema Operator

[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kafka-schema-operator)](https://artifacthub.io/packages/search?repo=kafka-schema-operator)

The Kafka Schema Operator delivers as easy way to deliver Kafka Schemas declaratively via Kubernetes `CRD`

Schema First approach should be implemented if you are using `schema-registry` according to [best practices](https://docs.confluent.io/platform/current/schema-registry/schema_registry_onprem_tutorial.html#viewing-schemas-in-schema-registry)

### Operator schema registry compatibility

* [Confluent](https://github.com/confluentinc/schema-registry)
* [Strimzi](https://github.com/lsst-sqre/strimzi-registry-operator)

### Operator features

* Create schemas declaratively via CRD
* Automatic schema versions update
* GitOps (automatic updates)

## Installation

```bash
helm repo add kafka-schema-operator https://pannoi.github.io/kafka-schema-operator-helm/
helm repo update
helm upgrade --install kafka-schema-operator kafka-schema-operator/kafka-schema-operator --values values.yaml
```

You need to set `SCHEMA_REGSITRY_HOST` and `SCHEMA_REGSITRY_PORT` initial functionality

```yaml
schemaRegistry:
  host:
  port:
```

If your schema registry needs API authentication add Key and Secret values in addition to host and port
```yaml
schemaRegistry:
  apiKey:
  apiSecret:
```

> You can refer from `secretRef`

> For more values check default [values](kubernetes/values.yaml)

## How to use

Deploy __ConfigMap__

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kafka-schema
  namespace: default
data:
  schema: |
    {
      "namespace": "testing",
      "type": "record",
      "name": "testing",
      "fields": [
        {"name": "id", "type": "string"},
        {"name": "email", "type": "string"}
      ]
    }

```

Refer to created ConfigMap via CustomResource __KafkaSchema__

```yaml
apiVersion: kafka-schema-operator.pannoi/v1beta1
kind: KafkaSchema
metadata:
  name: kafka-schema
  namespace: default
spec:
  name: testing
  schemaSerializer: string
  autoReconciliation: true # true = autoUpdate schema, false = for update CR should be re-created (not set => false)
  terminationProtection: true # true = don't delete resources on CR deletion, false = when CR deleted, deletes all resource: ConfigMap, Schema from registry (not set => false)
  data:
    configRef: kafka-schema # ConfigMap
    format: avro # avro/protobuf/json
    compatibility: # BACKWARD | BACKWARD_TRANSITIVE | FORWARD | FORWARD_TRANSITIVE | FULL | FULL_TRANSITIVE | NONE
```

> Resources (`KafkaSchema` & `ConfigMap`) should be located in same `namespace`
