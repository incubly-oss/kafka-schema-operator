package controller

import (
	"context"
	"fmt"
	"time"

	"incubly.oss/kafka-schema-operator/api/v1beta1"
	schemaregmock "incubly.oss/kafka-schema-operator/internal/schemareg-mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("KafkaSchema Controller", func() {

	BeforeEach(func() {
		srMock.Clear()
	})

	ctx := context.Background()
	Context("Naming strategies", func() {
		It("Should use SubjectName when strategy not defined", func() {
			By("When schema is created without naming strategy")
			expectedSubjectName := fmt.Sprintf("test-%d", time.Now().UnixMilli())
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				SubjectName: expectedSubjectName,
				Schema:      `"string"`,
				Format:      v1beta1.AVRO,
			})
			Ω(whenCreatingSchema(ctx, aSchema)).ShouldNot(BeNil())

			By("Then subject should have expected name")
			Expect(srMock.Subjects).Should(HaveKey(expectedSubjectName))
		})
		It("Should fail if strategy not provided and SubjectName missing", func() {
			By("When schema is created without naming strategy and missing SubjectName")
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				Schema: `"string"`,
				Format: v1beta1.AVRO,
			})
			_, err := whenCreatingSchema(ctx, aSchema)

			By("Then reconciliation should fail")
			Expect(err).Should(HaveOccurred())

			By("And subject shouldn't be registered")
			Expect(srMock.Subjects).Should(BeEmpty())
		})
		It("Should refer to TopicName in subject if TOPIC strategy", func() {
			By("When schema is created with TOPIC strategy")
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				NamingStrategy: v1beta1.TOPIC,
				TopicName:      "MY_TOPIC",
				Schema:         `"string"`,
				Format:         v1beta1.AVRO,
			})

			Ω(whenCreatingSchema(ctx, aSchema)).ShouldNot(BeNil())

			By("Then subject should have expected name")
			Expect(srMock.Subjects).Should(HaveKey("MY_TOPIC-value"))
		})
		It("Should fail if TOPIC strategy and TopicName missing", func() {
			By("When schema is created with valid TOPIC naming strategy but missing TopicName")
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				NamingStrategy: v1beta1.TOPIC,
				Schema:         `"string"`,
				Format:         v1beta1.AVRO,
			})
			_, err := whenCreatingSchema(ctx, aSchema)

			By("Then reconciliation should fail")
			Expect(err).Should(HaveOccurred())

			By("And subject shouldn't be registered")
			Expect(srMock.Subjects).Should(BeEmpty())
		})
		It("Should extract full record name from schema when RECORD strategy", func() {
			By("When schema is created with RECORD naming strategy")
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				NamingStrategy: v1beta1.RECORD,
				Schema:         `{"type":"record", "package":"foo.bar", "name":"BAZ", "other":"fields"}`,
				Format:         v1beta1.AVRO,
			})
			Ω(whenCreatingSchema(ctx, aSchema)).ShouldNot(BeNil())

			By("Then subject should have expected name")
			Expect(srMock.Subjects).Should(HaveKey("foo.bar.BAZ"))
		})
		It("Should use record name from schema when RECORD strategy and no package", func() {
			By("When schema is created with RECORD naming strategy")
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				NamingStrategy: v1beta1.RECORD,
				Schema:         `{"type":"record", "name":"BAZ", "other":"fields"}`,
				Format:         v1beta1.AVRO,
			})
			Ω(whenCreatingSchema(ctx, aSchema)).ShouldNot(BeNil())

			By("Then subject should have expected name")
			Expect(srMock.Subjects).Should(HaveKey("BAZ"))
		})
		It("Should fail if RECORD strategy and non-AVRO schema", func() {
			By("When schema is created with non-AVRO schema")
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				NamingStrategy: v1beta1.RECORD,
				Schema:         `{"type":"record", "name":"BAZ", "other":"fields"}`,
				Format:         v1beta1.JSON,
			})
			_, err := whenCreatingSchema(ctx, aSchema)

			By("Then reconciliation should fail")
			Expect(err).Should(HaveOccurred())

			By("And subject shouldn't be registered")
			Expect(srMock.Subjects).Should(BeEmpty())
		})
		It("Should fail if RECORD strategy and AVRO schema without name", func() {
			By("When schema is created with AVRO schema without name")
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				NamingStrategy: v1beta1.RECORD,
				Schema:         `{"type":"record", "package":"foo.bar", "other":"fields"}`,
				Format:         v1beta1.AVRO,
			})
			_, err := whenCreatingSchema(ctx, aSchema)

			By("Then reconciliation should fail")
			Expect(err).Should(HaveOccurred())

			By("And subject shouldn't be registered")
			Expect(srMock.Subjects).Should(BeEmpty())
		})
		It("Should fail if RECORD strategy and AVRO schema but not record", func() {
			By("When schema is created with AVRO schema but not representing record")
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				NamingStrategy: v1beta1.RECORD,
				Schema:         `{"package":"foo.bar", "name":"BAZ", "other":"fields"}`,
				Format:         v1beta1.AVRO,
			})
			_, err := whenCreatingSchema(ctx, aSchema)

			By("Then reconciliation should fail")
			Expect(err).Should(HaveOccurred())

			By("And subject shouldn't be registered")
			Expect(srMock.Subjects).Should(BeEmpty())
		})
		It("Should create subject when TOPIC_RECORD strategy", func() {
			By("When schema is created")
			topicName := fmt.Sprintf("test-%d", time.Now().UnixMilli())
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				NamingStrategy: v1beta1.TOPIC_RECORD,
				TopicName:      topicName,
				Schema:         `{"type":"record", "package":"foo.bar", "name":"BAZ", "other":"fields"}`,
				Format:         v1beta1.AVRO,
			})
			Ω(whenCreatingSchema(ctx, aSchema)).ShouldNot(BeNil())

			By("Then subject should have expected name")
			Expect(srMock.Subjects).Should(HaveKey(fmt.Sprintf("%s-foo.bar.BAZ", topicName)))
		})

	})

	Context("Cleanup policies", func() {

		It("Should do nothing if cleanup is DISABLED", func() {
			aSchema := aSchemaWithCleanupPolicy(v1beta1.DISABLED)
			By("When cleaning up schema with cleanup policy DISABLED")
			Ω(whenDeletingPreviouslyCreatedSchema(ctx, aSchema)).
				ShouldNot(BeNil())

			By("Then subject should NOT be deleted from registry")
			Expect(srMock.Subjects).Should(HaveLen(1))
		})
		It("Should soft-delete subject if cleanup is SOFT", func() {
			aSchema := aSchemaWithCleanupPolicy(v1beta1.SOFT)
			By("When cleaning up schema with cleanup policy SOFT")
			Ω(whenDeletingPreviouslyCreatedSchema(ctx, aSchema)).
				ShouldNot(BeNil())

			By("Then subject should be soft-deleted from registry")
			Expect(srMock.Subjects).Should(BeEmpty())
			Expect(srMock.SoftDeletedSubjects).Should(HaveLen(1))
		})
		It("Should soft-delete and hard-delete subject if cleanup is SOFT", func() {
			aSchema := aSchemaWithCleanupPolicy(v1beta1.HARD)
			By("When cleaning up schema with cleanup policy HARD")
			Ω(whenDeletingPreviouslyCreatedSchema(ctx, aSchema)).
				ShouldNot(BeNil())

			By("Then subject should be soft- and hard-deleted from registry")
			Expect(srMock.Subjects).Should(BeEmpty())
			Expect(srMock.SoftDeletedSubjects).Should(HaveLen(1))
			Expect(srMock.HardDeletedSubjects).Should(HaveLen(1))
		})
	})
	Context("Status", func() {
		It("Should update status on successful reconciliation", func() {
			By("When creating new schema")
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				SubjectName: "test",
				Format:      v1beta1.AVRO,
				Schema:      `"string"`,
			})

			Ω(whenCreatingSchema(ctx, aSchema)).
				ShouldNot(BeNil())

			By("Then resource status should be set")
			status := expectReadyConditionWithReason(ctx, aSchema, v1beta1.Complete)
			Expect(status.SchemaRegistryUrl).To(Equal(srMockServer.URL()))
			Expect(status.Subject).To(Equal(aSchema.Spec.SubjectName))
			Expect(status.SchemaId).To(BeNumerically(">", 0))
			Expect(status.Conditions).ToNot(BeEmpty())

			By("And status matches schema registry entries")
			Expect(srMock.Subjects).To(HaveKey(status.Subject))
			Expect(srMock.Schemas).To(HaveKeyWithValue(status.SchemaId, aSchema.Spec.Data.Schema))
		})
		It("Should update status on failed NamingStrategy", func() {
			By("When trying to create resource with invalid name strategy")
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				Format: v1beta1.AVRO,
				Schema: `"string"`,
			})
			_, err := whenCreatingSchema(ctx, aSchema)

			By("And reconciliation fails")
			Expect(err).ToNot(Succeed())

			By("Then failed ready condition should be set")
			expectReadyConditionWithReason(ctx, aSchema, v1beta1.NameStrategy)

		})
		It("Should update status on failed Cleanup", func() {
			By("Given DeleteSubject will fail in schema registry")
			srMock.InjectError(schemaregmock.InjectedError{
				OnApi:      schemaregmock.DeleteSubject,
				StatusCode: 500,
			})
			aSchema := aSchemaWithCleanupPolicy(v1beta1.SOFT)
			By("When trying to clean up resource")
			_, err := whenDeletingPreviouslyCreatedSchema(ctx, aSchema)

			By("And reconciliation fails")
			Expect(err).ToNot(Succeed())

			By("Then failed ready condition should be set")
			expectReadyConditionWithReason(ctx, aSchema, v1beta1.Cleanup)
		})
		It("Should update status on failed RegisterSchema", func() {
			By("When trying to create resource with invalid schema")
			aSchema := aSchemaWithNameStrategy(NameStrategy{
				SubjectName: "test",
				Format:      v1beta1.AVRO,
				Schema:      `invalid`,
			})
			_, err := whenCreatingSchema(ctx, aSchema)

			By("And reconciliation fails")
			Expect(err).ToNot(Succeed())

			By("Then failed ready condition should be set")
			expectReadyConditionWithReason(ctx, aSchema, v1beta1.RegisterSchema)
		})
		It("Should update status on failed SetCompatibilityMode", func() {
			By("Given SetCompatibilityMode will fails in schema registry")
			srMock.InjectError(schemaregmock.InjectedError{
				OnApi:      schemaregmock.SetCompatibilityMode,
				StatusCode: 500,
			})

			By("When trying to create new resource")
			aSchema := aSchemaWithCleanupPolicy(v1beta1.SOFT)
			_, err := whenCreatingSchema(ctx, aSchema)

			By("And reconciliation fails")
			Expect(err).ToNot(Succeed())

			By("Then failed ready condition should be set")
			expectReadyConditionWithReason(ctx, aSchema, v1beta1.SetCompatibilityMode)
		})
	})
})

func expectReadyConditionWithReason(ctx context.Context, schema *v1beta1.KafkaSchema, reason v1beta1.ReadyReason) v1beta1.KafkaSchemaStatus {
	current := &v1beta1.KafkaSchema{}

	ExpectWithOffset(1, k8sClient.SubResource("status").Get(ctx, schema, current)).
		To(Succeed())
	status := current.Status
	ExpectWithOffset(1, status.Conditions).ToNot(BeEmpty())

	ready := meta.FindStatusCondition(status.Conditions, "Ready")
	ExpectWithOffset(1, ready.Status).Should(BeEquivalentTo(reason.Status))
	ExpectWithOffset(1, ready.LastTransitionTime.IsZero()).
		To(BeFalseBecause("LastTransitionTime was not set"))
	ExpectWithOffset(1, ready.Reason).To(BeEquivalentTo(reason.Name))
	return status
}

func whenDeletingPreviouslyCreatedSchema(ctx context.Context, aSchema *v1beta1.KafkaSchema) (ctrl.Result, error) {

	lookupName := namespacedName(aSchema)

	cut := &KafkaSchemaReconciler{
		Client: k8sClient,
		Scheme: k8sClient.Scheme(),
	}

	Ω(whenCreatingSchema(ctx, aSchema)).ShouldNot(BeNil())

	By("-- verifying that subjects were created")
	ExpectWithOffset(1, srMock.Subjects).Should(HaveLen(1))

	By("-- deleting schema")
	ExpectWithOffset(1, k8sClient.Delete(ctx, aSchema)).
		To(Succeed())
	ExpectWithOffset(1, k8sClient.Get(ctx, lookupName, aSchema)).
		To(Succeed())
	ExpectWithOffset(1, aSchema.DeletionTimestamp.IsZero()).
		To(BeFalseBecause("Resource was not marked for deletion"))

	return cut.Reconcile(ctx, reconcile.Request{NamespacedName: lookupName})
}

func whenCreatingSchema(ctx context.Context, aSchema *v1beta1.KafkaSchema) (ctrl.Result, error) {
	lookupName := namespacedName(aSchema)
	cut := &KafkaSchemaReconciler{
		Client: k8sClient,
		Scheme: k8sClient.Scheme(),
	}

	By("-- creating schema")
	ExpectWithOffset(1, k8sClient.Create(ctx, aSchema)).
		To(Succeed())
	ExpectWithOffset(1, k8sClient.Get(ctx, lookupName, aSchema)).
		To(Succeed())
	return cut.Reconcile(ctx, reconcile.Request{NamespacedName: lookupName})
}

func namespacedName(resource *v1beta1.KafkaSchema) types.NamespacedName {
	return types.NamespacedName{Namespace: resource.Namespace, Name: resource.Name}
}

func aSchemaWithCleanupPolicy(policy v1beta1.CleanupPolicy) *v1beta1.KafkaSchema {
	return &v1beta1.KafkaSchema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("test-%d", time.Now().UnixMilli()),
			Namespace: "default",
		},
		Spec: v1beta1.KafkaSchemaSpec{
			SubjectName:   "test",
			CleanupPolicy: policy,
			Data: v1beta1.KafkaSchemaData{
				Schema:        `"string"`,
				Format:        v1beta1.AVRO,
				Compatibility: v1beta1.BACKWARD,
			},
		},
	}
}

type NameStrategy struct {
	NamingStrategy v1beta1.NamingStrategy
	SubjectName    string
	TopicName      string
	Schema         string
	Format         v1beta1.SchemaFormat
}

func aSchemaWithNameStrategy(args NameStrategy) *v1beta1.KafkaSchema {
	return &v1beta1.KafkaSchema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("test-%d", time.Now().UnixMilli()),
			Namespace: "default",
		},
		Spec: v1beta1.KafkaSchemaSpec{
			NamingStrategy: args.NamingStrategy,
			SubjectName:    args.SubjectName,
			TopicName:      args.TopicName,
			CleanupPolicy:  v1beta1.DISABLED,
			Data: v1beta1.KafkaSchemaData{
				Schema:        args.Schema,
				Format:        args.Format,
				Compatibility: v1beta1.NONE,
			},
		},
	}
}
