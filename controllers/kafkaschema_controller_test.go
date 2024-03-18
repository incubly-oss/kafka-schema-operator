package controllers

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"kafka-schema-operator/api/v2beta1"
	schemareg_mock "kafka-schema-operator/schemareg-mock"
	"strconv"
	"time"
)

var _ = Describe("KafkaschemaController", func() {

	BeforeEach(func() {
		srMock.Clear()
	})
	ctx := context.Background()

	Context("When creating resource", func() {

		It("Should register subject+schema and manage resource status", func() {
			topicName := "testenv.testservice.testevent." + strconv.FormatInt(time.Now().UnixMilli(), 10)

			By("When: creating KafkaSchema resource")
			Expect(
				k8sClient.Create(ctx, aKafkaSchemaResource(topicName)),
			).Should(Succeed())

			By("Then: resource is properly created in K8S")
			Expect(currentResourceState(ctx, topicName).Spec.Name).Should(Equal(topicName))
			// TODO: verify resource

			// TODO: switch to observing resource status and swap those 2 steps
			By("And: subjects and schemas for topic key & value are registered in schema registry")
			assertSubjectsCreatedInSchemaRegistry(topicName+"-key", topicName+"-value")
			Expect(srMock.Subjects[topicName+"-value"].SchemaRefs).Should(HaveLen(1))
			// TODO: verify schema

			By("And: resource are updated with proper status and finalizer")
			Expect(
				currentResourceState(ctx, topicName).Finalizers,
			).Should(
				ConsistOf("kafka-schema-operator.incubly/finalizer"),
			)
			// TODO: verify status (& events?)
		})
	})
	Context("When updating resource", func() {
		PIt("Should update schema payload", func() {
		})
		PIt("Should update compatibility mode", func() {
		})
		PIt("Should fail if schema change is incompatible", func() {
		})
	})
	Context("When deleting resource", func() {
		It("Should soft-delete subject and cleanup resource", func() {
			topicName := "testenv.testservice.testevent." + strconv.FormatInt(time.Now().UnixMilli(), 10)
			schemaResource := aKafkaSchemaResource(topicName)

			By("Given: KafkaSchema resource was already deployed")
			Expect(
				k8sClient.Create(ctx, schemaResource),
			).Should(Succeed())
			By("And: controller handled resource creation correctly")
			// TODO: switch to observing resource status instead of waiting for schema registry
			assertSubjectsCreatedInSchemaRegistry(topicName+"-key", topicName+"-value")

			By("When: deleting resource")
			Expect(k8sClient.Delete(ctx, schemaResource)).Should(Succeed())

			By("And: resource is eventually deleted")
			/*
				We're waiting for the controller to reconcile the resource
				(marked for deletion, because it had finalizer - see previous test).
			*/
			// TODO: verify the error type & reason
			Eventually(func() error {
				return k8sClient.Get(ctx, resourceLookupKey(topicName), &v2beta1.KafkaSchema{})
			}).ShouldNot(Succeed())

			By("Then: both subjects are soft-deleted from schema registry")
			Expect(srMock.Subjects).ShouldNot(Or(
				HaveKey(topicName+"-key"),
				HaveKey(topicName+"-value")))
		})
	})
})

func assertSubjectsCreatedInSchemaRegistry(subjectNames ...string) {
	subjectNameMatchers := []types.GomegaMatcher{}
	for _, subjectName := range subjectNames {
		subjectNameMatchers = append(subjectNameMatchers, HaveKey(subjectName))
	}
	Eventually(func() map[string]*schemareg_mock.Subject {
		return srMock.Subjects
	},
		time.Second*10,
		time.Millisecond*100,
	).Should(And(subjectNameMatchers...))
}

func currentResourceState(ctx context.Context, topicName string) *v2beta1.KafkaSchema {
	res := &v2beta1.KafkaSchema{}
	Eventually(func() error {
		return k8sClient.Get(ctx, resourceLookupKey(topicName), res)
	}).Should(Succeed())
	return res
}
