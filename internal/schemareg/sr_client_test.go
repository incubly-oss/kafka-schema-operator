package schemareg

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"

	"incubly.oss/kafka-schema-operator/api/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("SrClient", func() {

	Context("When creating client", func() {

		BeforeEach(func() {
			_ = os.Setenv("SCHEMA_REGISTRY_BASE_URL", "https://default.host:6666")

			//os.Unsetenv("SCHEMA_REGISTRY_KEY")
			//os.Unsetenv("SCHEMA_REGISTRY_SECRET")
		})

		It("Should use provided BaseUrl", func() {
			client, err := NewClient(&v1beta1.SchemaRegistry{
				BaseUrl: "http://my.host:1234",
			}, log.Log)
			Expect(err).Should(Succeed())
			Expect(client.BaseUrl.String()).Should(Equal("http://my.host:1234"))
		})
		It("Should fall back to default BaseUrl", func() {
			client, err := NewClient(nil, log.Log)
			Expect(err).Should(Succeed())
			Expect(client.BaseUrl.String()).Should(Equal("https://default.host:6666"))
		})
		//It("Should use provided basic auth credentials", func() {
		//	client, err := NewClient(&v1beta1.SchemaRegistry{
		//		Key:    "myKey",
		//		Secret: "mySecret",
		//	}, log.Log)
		//	Expect(err).Should(Succeed())
		//	Expect(client.basicAuthCreds.user).Should(Equal("myKey"))
		//	Expect(client.basicAuthCreds.pass).Should(Equal("mySecret"))
		//})
		//It("Should fall back to default basic auth credentials", func() {
		//	os.Setenv("SCHEMA_REGISTRY_KEY", "defaultKey")
		//	os.Setenv("SCHEMA_REGISTRY_SECRET", "defaultSecret")
		//	client, err := NewClient(nil, log.Log)
		//	Expect(err).Should(Succeed())
		//	Expect(client.basicAuthCreds.user).Should(Equal("defaultKey"))
		//	Expect(client.basicAuthCreds.pass).Should(Equal("defaultSecret"))
		//
		//})
		//It("Should work without basic auth credentials", func() {
		//	client, err := NewClient(nil, log.Log)
		//	Expect(err).Should(Succeed())
		//	Expect(client.basicAuthCreds).To(BeNil())
		//})
	})
	Context("When using client", func() {
		var collectedRequests []*http.Request
		schemaRegMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			collectedRequests = append(collectedRequests, r.Clone(r.Context()))
			if isRegisterSchemaRequest(r) {
				_, _ = w.Write([]byte(`{"id": -1234}`))
			}
			w.WriteHeader(200)
		}))
		//defer schemaRegMock.Close()

		schemaRegMockUrl, err := url.Parse(schemaRegMock.URL)
		Expect(err).Should(Succeed())

		clientUnderTest := SrClient{
			BaseUrl: schemaRegMockUrl,
			logger:  log.Log,
		}
		BeforeEach(func() {
			collectedRequests = []*http.Request{}
		})

		It("Should register schema under subject", func() {
			res, err := clientUnderTest.RegisterSchema("mysubject", RegisterSchemaReq{
				Schema:     `{"type": "record","name": "test","fields":[{"type": "string","name": "field1"}]}`,
				SchemaType: v1beta1.AVRO,
			})
			Expect(err).Should(Succeed())
			Expect(res).Should(Equal(-1234))

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
			Expect(clientUnderTest.DeleteSubject("mysubject", false)).Should(Succeed())
			Expect(collectedRequests).To(HaveLen(1))
			actualReq := collectedRequests[0]
			Expect(actualReq.URL.Path).Should(Equal("/subjects/mysubject"))
			Expect(actualReq.Method).Should(Equal("DELETE"))
			//Expect(actualReq.Body).Should(BeNil())
			Expect(actualReq.Header).Should(HaveKeyWithValue("Content-Type", []string{"application/vnd.schemaregistry.v1+json"}))
		})
		It("Should set compatibility mode", func() {
			Expect(
				clientUnderTest.SetCompatibilityMode(
					"mysubject",
					SetCompatibilityModeReq{
						//Compatibility: "BACKWARD",
					},
				),
			).Should(Succeed())

			Expect(collectedRequests).To(HaveLen(1))
			actualReq := collectedRequests[0]
			Expect(actualReq.URL.Path).Should(Equal("/config/mysubject"))
			Expect(actualReq.Method).Should(Equal("PUT"))
			//Expect(io.ReadAll(actualReq.Body)).Should(Equal("BACKWARD"))
			Expect(actualReq.Header).Should(HaveKeyWithValue("Content-Type", []string{"application/vnd.schemaregistry.v1+json"}))
		})
		//It("Should not send basic auth if client has no auth", func() {
		//	Expect(
		//		clientUnderTest.SetCompatibilityMode(
		//			"mysubject",
		//			SetCompatibilityModeReq{
		//				Compatibility: "BACKWARD",
		//			},
		//		),
		//	).Should(Succeed())
		//	Expect(collectedRequests).To(HaveLen(1))
		//	actualReq := collectedRequests[0]
		//	Expect(actualReq.Header).ShouldNot(HaveKey("Authorization"))
		//})
		//It("Should support basic auth", func() {
		//	clientWithBasicAuth := SrClient{
		//		BaseUrl:        schemaRegMock.URL,
		//		basicAuthCreds: &BasicAuthCreds{"user", "pass"},
		//		logger:         log.Log,
		//	}
		//	Expect(clientWithBasicAuth.DeleteSubject("mysubject", false)).Should(Succeed())
		//	Expect(collectedRequests).To(HaveLen(1))
		//	actualReq := collectedRequests[0]
		//	Expect(actualReq.Header).Should(HaveKeyWithValue(
		//		"Authorization",
		//		[]string{"Basic " + b64.StdEncoding.EncodeToString([]byte("user:pass"))}))
		//})
	})
})

func isRegisterSchemaRequest(r *http.Request) bool {
	return r.Method == "POST" && strings.Contains(r.URL.Path, "/versions")
}
