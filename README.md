# Kafka Schema Operator

This is a fork of [Pannoi kafka-schema-operator](https://github.com/pannoi/kafka-schema-operator).
Kudos to author for maintaining it.
The reason we've decided to fork the repository
was the necessity to introduce backward-incompatible change in resource structure.
See [this PR](https://github.com/Tomek-Adamczewski/kafka-schema-operator/pull/1) for details.

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
helm repo add incubly-kafka-schema-operator https://tomek-adamczewski.github.io/kafka-schema-operator/
helm repo update
helm upgrade --install incubly-kafka-schema-operator incubly-kafka-schema-operator/kafka-schema-operator --values values.yaml
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

```yaml
apiVersion: kafka-schema-operator.incubly/v2beta1
kind: KafkaSchema
metadata:
  name: kafka-schema
  namespace: default
schemaRegistry:
  host: ...
  port: ...
  key: ...
  secret: ...
spec:
  name: testing
  schemaSerializer: string
  autoReconciliation: true # true = autoUpdate schema, false = for update CR should be re-created (not set => false)
  terminationProtection: true # true = don't delete resources on CR deletion, false = when CR deleted, deletes all resource: ConfigMap, Schema from registry (not set => false)
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
    format: avro # avro/protobuf/json
    compatibility: # BACKWARD | BACKWARD_TRANSITIVE | FORWARD | FORWARD_TRANSITIVE | FULL | FULL_TRANSITIVE | NONE
```
