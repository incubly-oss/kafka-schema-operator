# kafka-schema-operator

Kubernetes operator managing
for [Schema Registry](https://docs.confluent.io/platform/current/schema-registry/index.html)
subjects and schemas. It was created because there were no alternatives that allowed
IaC management of Schema Registry in microservice/autonomous fashion.

It was inspired by [Anton's operator](https://github.com/pannoi/kafka-schema-operator),
but we've taken a slightly different approach to the problem:

* Single KafkaSchema resource reflects a single Schema Registry subject.
  This allows fine-grained control of the schemas you manage
* It respects
  [Confluent's Subject name strategy](https://docs.confluent.io/platform/current/schema-registry/fundamentals/serdes-develop/index.html#subject-name-strategy)
* You can define schema directly in the resource. There's no need to manage
  and synchronize multiple K8S resources,  which makes it easier to use with
  Helm Charts etc...

## Description

Operator is responsible for managing state of the KafkaSchema resource.
Basic resource structure:

```yaml
apiVersion: kafka.incubly.oss/v1beta1
kind: KafkaSchema
metadata:
  name: my-schema
spec:
  namingStrategy: "io.confluent.kafka.serializers.subject.TopicNameStrategy"
  topicName: "my-topic"
  cleanupPolicy: "SOFT"
  schemaRegistry:
    baseUrl: "https://my-schema-registry:8081"
  data:
    format: "AVRO"
    schema: |
      {
        "namespace": "example.avro",
        "type": "record",
        "name": "User",
        "fields": [
          {"name": "name", "type": "string"},
          {"name": "favorite_number",  "type": ["int", "null"]},
          {"name": "favorite_color", "type": ["string", "null"]}
        ]
      }
```

### Naming Strategy

You can either specify exact subject name using `.spec.subjectName`,
or configure one of the naming strategies and let operator manage subject names for you:
* io.confluent.kafka.serializers.subject.TopicNameStrategy: `<topicName>-value`
* io.confluent.kafka.serializers.subject.RecordNameStrategy: `<avroRecordName>`
* io.confluent.kafka.serializers.subject.TopicRecordNameStrategy: `<topicName>-<avroRecordName>`

Where: topicName is defined in `.spec.topicName` (mandatory for naming strategies that use it), and avroRecordName is extracted from Avro record schema, hence it only works with `spec.data.format=AVRO` and if schema is a valid Avro record (with `type=record`, mandatory `name` and optional `package`).

`.spec.namingStrategy` values matches `value.subject.name.strategy` KafkaOption,
making it easy to configure KafkaSchemas and deployments from the same Helm values.

More details: [Confluent documentation](https://docs.confluent.io/platform/current/schema-registry/fundamentals/serdes-develop/index.html#subject-name-strategy).

> [!TIP]
> You can configure your templates to always populate both `topicName` and `subjectName` and,
> depending on `namingStrategy`, operator will use appropriate value.

### Cleanup Strategy

Defines what should happen when KafkaSchema resource is being deleted:
* DISABLED - won't try to remove subject and schema from registry.
  Useful for "higher/stable environments" (e.g. production),
  where you'd rather not remove topics and schemas
* SOFT - will soft-delete subject from registry
* HARD - will hard-delete subject and schema from registry.
  Useful for dynamic/temporary/ephemeral environments connected to stable broker/registry.
 

More details: [Confluent documentation](https://docs.confluent.io/platform/current/schema-registry/schema-deletion-guidelines.html).

### Schema Registry

Allows to override operator's default configuration of Schema Registry.
It's useful in setups where you have multiple environments,
each with its dedicated Schema Registry instance,
all managed by a single cluster-wide operator.

You can check Schema Registry associated with specific KafkaSchema
(regardless if it's explicitly configured for that resource or default configuration was used)
in `.status.schemaRegistryUrl` field of healthy resource. 

### Data Format and Schema

Operator supports all Schema Registry formats: AVRO, JSON and PROTOBUF.
It won't verify correctness of the schema (except for Record and TopicRecord
naming strategies, where operator needs to parse Avro schema to extract record name).
However, Schema Registry does and resources with invalid schemas
(or format not matching provided schema) will fail to register schemas.

### Compatibility Mode

Additionally, you can define `.spec.data.compatibility` for each resource.
If compatibility mode is not defined for specific subject, Schema Registry
will use its global mode when verifying new versions of schema.

More details: [Confluent documentation](https://docs.confluent.io/platform/current/schema-registry/fundamentals/schema-evolution.html#compatibility-types)

### Resource Status

Operator maintains resource status will useful information about synchronization state
of the resource, as well as single condition of type `"Ready"`.

More details: [CRD (status)](./helm-charts/crds/kafka.incubly.oss_kafkaschemas.yaml).

## Getting Started

You can install the operator using Helm:

```shell
helm repo update
helm repo add kafka-schema-operator https://incubly-oss.github.io/kafka-schema-operator/
helm upgrade --install kafka-schema-operator kafka-schema-operator/kafka-schema-operator --values values.yaml
```

Config options: see comments in [default values](./helm-charts/values.yaml).

## Contributing

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)
