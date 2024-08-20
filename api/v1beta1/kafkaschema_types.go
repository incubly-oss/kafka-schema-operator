package v1beta1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:validation:Enum=DISABLED;SOFT;HARD
type CleanupPolicy string

const (
	DISABLED CleanupPolicy = "DISABLED"
	SOFT     CleanupPolicy = "SOFT"
	HARD     CleanupPolicy = "HARD"
)

// +kubebuilder:validation:Enum=io.confluent.kafka.serializers.subject.TopicNameStrategy;io.confluent.kafka.serializers.subject.RecordNameStrategy;io.confluent.kafka.serializers.subject.TopicRecordNameStrategy
type NamingStrategy string

const (
	TOPIC        NamingStrategy = "io.confluent.kafka.serializers.subject.TopicNameStrategy"
	RECORD       NamingStrategy = "io.confluent.kafka.serializers.subject.RecordNameStrategy"
	TOPIC_RECORD NamingStrategy = "io.confluent.kafka.serializers.subject.TopicRecordNameStrategy"
)

// +kubebuilder:validation:Enum=AVRO;JSON;PROTOBUF
type SchemaFormat string

const (
	AVRO     SchemaFormat = "AVRO"
	JSON     SchemaFormat = "JSON"
	PROTOBUF SchemaFormat = "PROTOBUF"
)

// +kubebuilder:validation:Enum=NONE;BACKWARD;BACKWARD_TRANSITIVE;FORWARD;FORWARD_TRANSITIVE;FULL;FULL_TRANSITIVE
type CompatibilityMode string

const (
	NONE                CompatibilityMode = "NONE"
	BACKWARD            CompatibilityMode = "BACKWARD"
	BACKWARD_TRANSITIVE CompatibilityMode = "BACKWARD_TRANSITIVE"
	FORWARD             CompatibilityMode = "FORWARD"
	FORWARD_TRANSITIVE  CompatibilityMode = "FORWARD_TRANSITIVE"
	FULL                CompatibilityMode = "FULL"
	FULL_TRANSITIVE     CompatibilityMode = "FULL_TRANSITIVE"
)

type KafkaSchemaData struct {
	// Schema payload. Format depends on associated "format" field
	Schema string `json:"schema"`
	// Format of the provided schema
	Format SchemaFormat `json:"format"`
	/*
		Compatibility defines schema compatibility mode for the subject.
		If not provided, subject will inherit default compatibility mode defined in schema registry
		See [official Confluent documentation](https://docs.confluent.io/platform/current/schema-registry/fundamentals/schema-evolution.html)
		for details.
	*/
	// +kubebuilder:validation:Enum=NONE;BACKWARD;BACKWARD_TRANSITIVE;FORWARD;FORWARD_TRANSITIVE;FULL;FULL_TRANSITIVE
	Compatibility CompatibilityMode `json:"compatibility,omitempty"`

	/*
		Should Operator normalize the schema.
		https://docs.confluent.io/platform/current/schema-registry/fundamentals/serdes-develop/index.html#schema-normalization
		https://avro.apache.org/docs/1.11.1/specification/#parsing-canonical-form-for-schemas
		Currently supported only for AVRO. Otherwise, it's ignored
	*/
	Normalize bool `json:"normalize,omitempty"`
}

type SchemaRegistry struct {
	/*
		BaseUrl of the schema registry this schema should be registered to.
		If not provided, controller will fall back to default configuration
	*/
	BaseUrl string `json:"baseUrl,omitempty"`
}

// KafkaSchemaSpec defines the desired state of KafkaSchema
type KafkaSchemaSpec struct {
	/*
		NamingStrategy is used to define name for the schema subject.
		It follows the [Confluent subject name strategy](https://docs.confluent.io/platform/current/schema-registry/fundamentals/serdes-develop/index.html#subject-name-strategy).

		Possible values:
		io.confluent.kafka.serializers.subject.TopicNameStrategy: uses [TopicNameStrategy](https://github.com/marcintustin/kafka-connect-json-avro-converter/blob/master/avro-serializer/src/main/java/io/confluent/kafka/serializers/subject/TopicNameStrategy.java#L35). Subject will have name "<TopicName>-value"
		io.confluent.kafka.serializers.subject.RecordNameStrategy: uses [RecordNameStrategy](https://github.com/marcintustin/kafka-connect-json-avro-converter/blob/master/avro-serializer/src/main/java/io/confluent/kafka/serializers/subject/RecordNameStrategy.java#L58). Subject will have name "<RecordName>". NOTE: It only works with AVRO records/schemas!
		io.confluent.kafka.serializers.subject.TopicRecordNameStrategy: uses [TopicRecordNameStrategy](https://github.com/marcintustin/kafka-connect-json-avro-converter/blob/master/avro-serializer/src/main/java/io/confluent/kafka/serializers/subject/TopicRecordNameStrategy.java#L39). It's similar to RECORD, but prefixes the subject name with topic: "<TopicName>-<RecordName>"

		If not provided, operator will try to create subject with name defined by SubjectName.
	*/
	NamingStrategy NamingStrategy `json:"namingStrategy,omitempty"`
	// SubjectName is mandatory if NamingStrategy is not provided. Otherwise, it's ignored
	SubjectName string `json:"subjectName,omitempty"`
	// TopicName is mandatory if NamingStrategy is set to "Topic" or "TopicRecord". Otherwise, it's ignored
	TopicName string `json:"topicName,omitempty"`

	/*
		CleanupPolicy defines interaction with schema registry when resource is deleted:
		DISABLED: no effect - controller won't attend to remove schemas and subjects
		SOFT: soft deletion - controller will delete subjects but leave schemas untouched
		HARD: hard deletion - controller will delete subjects and referenced schemas. NOTE: if schema is referenced by another subject, schema registry won't effectively delete it

		If not provided, controller will fall back to its default (configurable) behaviour
	*/
	CleanupPolicy CleanupPolicy `json:"cleanupPolicy,omitempty"`
	/*
		SchemaRegistry optionally overrides controller default reference to schema registry it targets
	*/
	SchemaRegistry SchemaRegistry `json:"schemaRegistry,omitempty"`
	/*
		Schema isa definition of schema associated with Kafka topic value.
		It's mandatory since Controller assumes that you primarily want to use
		associated custom resources for schemas representing topic values.
	*/
	Data KafkaSchemaData `json:"data"`
}

// KafkaSchemaStatus defines the observed state of KafkaSchema
type KafkaSchemaStatus struct {
	// Represents observations of the current state of KafkaSchema.
	// Operator uses single condition with type="Ready" and statuses:
	// True (reconciliation complete), False (reconciliation failed)
	// and Unknown (reconciliation in progress).
	//
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// SchemaRegistryUrl is an effective URL of the schema registry this resource interacts with
	SchemaRegistryUrl string `json:"schemaRegistryUrl,omitempty"`
	// SchemaId is the identifier of the schema in the schema registry
	SchemaId int `json:"keySchemaId,omitempty"`
	// Subject is the schema registry subject (based on NamingStrategy)
	Subject string `json:"subject,omitempty"`
	// Healthy boolean reflects current health of the resource
	Healthy bool `json:"healthy,omitempty"`
	// Status is equivalent to Healthy, but with format based on pod status
	Status string `json:"status,omitempty"`
	// RetryCount is incremented on any subsequent failure (and reset to 0 on each success)
	RetryCount int `json:"retryCount,omitempty"`
	// LastRetryTsEpoch timestamp of last retry attempt, in epoch millis
	LastRetryTsEpoch int64 `json:"lastRetryTsEpoch,omitempty"`
}

type ReadyReason struct {
	Name   string
	Status metav1.ConditionStatus
}

var (
	InProgress           = ReadyReason{"InProgress", metav1.ConditionUnknown}
	Complete             = ReadyReason{"Complete", metav1.ConditionTrue}
	NameStrategy         = ReadyReason{"NameStrategy", metav1.ConditionFalse}
	SchemaRegistryClient = ReadyReason{"SchemaRegistryClient", metav1.ConditionFalse}
	NormalizeSchema      = ReadyReason{"NormalizeSchema", metav1.ConditionFalse}
	RegisterSchema       = ReadyReason{"RegisterSchema", metav1.ConditionFalse}
	ResourceUpdate       = ReadyReason{"ResourceUpdate", metav1.ConditionFalse}
	SetCompatibilityMode = ReadyReason{"SetCompatibilityMode", metav1.ConditionFalse}
	Cleanup              = ReadyReason{"Cleanup", metav1.ConditionFalse}
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KafkaSchema is the Schema for the kafkaschemas API
type KafkaSchema struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaSchemaSpec   `json:"spec,omitempty"`
	Status KafkaSchemaStatus `json:"status,omitempty"`
}

func (in *KafkaSchema) SetReadyReason(reason ReadyReason, msg string) bool {
	return meta.SetStatusCondition(
		&in.Status.Conditions,
		metav1.Condition{
			Type:               "Ready",
			Status:             reason.Status,
			ObservedGeneration: in.Generation,
			Reason:             reason.Name,
			Message:            msg,
		},
	)
}

//+kubebuilder:object:root=true

// KafkaSchemaList contains a list of KafkaSchema
type KafkaSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaSchema `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaSchema{}, &KafkaSchemaList{})
}
