package controllers

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"kafka-schema-operator/api/v2beta1"
	schemareg_mock "kafka-schema-operator/schemareg-mock"
	"strconv"
	"time"
)

const ResourceNs = "default"

var _ = Describe("KafkaschemaController", func() {

	BeforeEach(func() {
		srMock.Clear()
	})
	Context("When creating Schema", func() {

		It("Should create schema", func() {
			topicName := "testenv.testservice.testevent." + strconv.FormatInt(time.Now().UnixMilli(), 10)
			schemaRes := v2beta1.KafkaSchema{
				TypeMeta: metav1.TypeMeta{
					Kind:       "KafkaSchema",
					APIVersion: "kafka-schema-operator.incubly/v2beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
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

			By("When: creating KafkaSchema resource")
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, &schemaRes)).Should(Succeed())

			By("Then: resource is created in K8S")
			schemaLookupKey := types.NamespacedName{Name: topicName, Namespace: ResourceNs}
			createdResource := &v2beta1.KafkaSchema{}
			Eventually(func() error {
				return k8sClient.Get(ctx, schemaLookupKey, createdResource)
			}).Should(Succeed())

			Expect(createdResource.Spec.Name).Should(Equal(topicName))
			// TODO: verify resource

			By("And: subjects and schemas for topic key & value are registered in schema registry")

			Eventually(func() map[string]*schemareg_mock.Subject {
				return srMock.Subjects
			},
				time.Second*10,
				time.Millisecond*100,
			).Should(
				And(
					HaveKey(topicName+"-key"),
					HaveKey(topicName+"-value"),
				),
			)
			Expect(srMock.Subjects[topicName+"-value"].SchemaRefs).Should(HaveLen(1))
			// TODO: verify schema
		})

	})
})
