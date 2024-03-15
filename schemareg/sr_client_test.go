package schemareg

import (
	b64 "encoding/base64"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"kafka-schema-operator/api/v2beta1"
	"net/http"
	"net/http/httptest"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("SrClient", func() {

	Context("When creating client", func() {

		BeforeEach(func() {
			os.Setenv("SCHEMA_REGISTRY_HOST", "default.host")
			os.Setenv("SCHEMA_REGISTRY_PORT", "6666")

			os.Unsetenv("SCHEMA_REGISTRY_KEY")
			os.Unsetenv("SCHEMA_REGISTRY_SECRET")
		})

		It("Should use provided baseUrl", func() {
			client, err := NewClient(&v2beta1.SchemaRegistry{
				Host: "my.host",
				Port: 1234,
			}, log.Log)
			Expect(err).Should(Succeed())
			Expect(client.baseUrl).Should(Equal("http://my.host:1234"))
		})
		It("Should fall back to default baseUrl", func() {
			client, err := NewClient(nil, log.Log)
			Expect(err).Should(Succeed())
			Expect(client.baseUrl).Should(Equal("http://default.host:6666"))
		})
		It("Should use provided basic auth credentials", func() {
			client, err := NewClient(&v2beta1.SchemaRegistry{
				Key:    "myKey",
				Secret: "mySecret",
			}, log.Log)
			Expect(err).Should(Succeed())
			Expect(client.basicAuthCreds.user).Should(Equal("myKey"))
			Expect(client.basicAuthCreds.pass).Should(Equal("mySecret"))
		})
		It("Should fall back to default basic auth credentials", func() {
			os.Setenv("SCHEMA_REGISTRY_KEY", "defaultKey")
			os.Setenv("SCHEMA_REGISTRY_SECRET", "defaultSecret")
			client, err := NewClient(nil, log.Log)
			Expect(err).Should(Succeed())
			Expect(client.basicAuthCreds.user).Should(Equal("defaultKey"))
			Expect(client.basicAuthCreds.pass).Should(Equal("defaultSecret"))

		})
		It("Should work without basic auth credentials", func() {
			client, err := NewClient(nil, log.Log)
			Expect(err).Should(Succeed())
			Expect(client.basicAuthCreds).To(BeNil())
		})
	})
	Context("When using client", func() {
		var collectedRequests []*http.Request
		schemaRegMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			collectedRequests = append(collectedRequests, r.Clone(r.Context()))
		}))
		//defer schemaRegMock.Close()
		clientUnderTest := SrClient{
			baseUrl: schemaRegMock.URL,
			logger:  log.Log,
		}
		BeforeEach(func() {
			collectedRequests = []*http.Request{}
		})

		It("Should register schema under subject", func() {
			Expect(clientUnderTest.RegisterSchema("mysubject", RegisterSchemaReq{
				Schema:     `{"type": "record","name": "test","fields":[{"type": "string","name": "field1"}]}`,
				SchemaType: "AVRO",
			})).Should(Succeed())

			expectedPayload := `
			{
			  "schema": "{
			    \"type\": \"record\",
			    \"name\": \"test\",
			    \"fields\": [
			      { \"type\": \"string\", \"name\": \"field1\" }
			    ]
			  }",
			  "schemaType": "AVRO",
			}
			`

			Expect(collectedRequests).To(HaveLen(1))
			actualReq := collectedRequests[0]
			Expect(actualReq.URL.Path).Should(Equal("/subjects/mysubject/versions"))
			Expect(actualReq.Method).Should(Equal("POST"))
			//Expect(io.ReadAll(actualReq.Body)).Should(Equal(expectedPayload))
			Expect(expectedPayload).Should(Equal(expectedPayload))
			Expect(actualReq.Header).Should(HaveKeyWithValue("Content-Type", []string{"application/vnd.schemaregistry.v1+json"}))
		})
		It("Should soft-delete subject", func() {
			Expect(clientUnderTest.DeleteSubject("mysubject")).Should(Succeed())
			Expect(collectedRequests).To(HaveLen(1))
			actualReq := collectedRequests[0]
			Expect(actualReq.URL.Path).Should(Equal("/subjects/mysubject"))
			Expect(actualReq.Method).Should(Equal("DELETE"))
			//Expect(actualReq.Body).Should(BeNil())
			Expect(actualReq.Header).Should(HaveKeyWithValue("Content-Type", []string{"application/vnd.schemaregistry.v1+json"}))
		})
		It("Should set compatibility mode", func() {
			Expect(clientUnderTest.SetCompatibilityMode("mysubject", "BACKWARD")).Should(Succeed())
			Expect(collectedRequests).To(HaveLen(1))
			actualReq := collectedRequests[0]
			Expect(actualReq.URL.Path).Should(Equal("/config/mysubject"))
			Expect(actualReq.Method).Should(Equal("PUT"))
			//Expect(io.ReadAll(actualReq.Body)).Should(Equal("BACKWARD"))
			Expect(actualReq.Header).Should(HaveKeyWithValue("Content-Type", []string{"application/vnd.schemaregistry.v1+json"}))
		})
		It("Should not send basic auth if client has no auth", func() {
			Expect(clientUnderTest.SetCompatibilityMode("mysubject", "BACKWARD")).Should(Succeed())
			Expect(collectedRequests).To(HaveLen(1))
			actualReq := collectedRequests[0]
			Expect(actualReq.Header).ShouldNot(HaveKey("Authorization"))
		})
		It("Should support basic auth", func() {
			clientWithBasicAuth := SrClient{
				baseUrl:        schemaRegMock.URL,
				basicAuthCreds: &BasicAuthCreds{"user", "pass"},
				logger:         log.Log,
			}
			Expect(clientWithBasicAuth.SetCompatibilityMode("mysubject", "BACKWARD")).Should(Succeed())
			Expect(collectedRequests).To(HaveLen(1))
			actualReq := collectedRequests[0]
			Expect(actualReq.Header).Should(HaveKeyWithValue(
				"Authorization",
				[]string{"Basic " + b64.StdEncoding.EncodeToString([]byte("user:pass"))}))
		})
	})
})
