package schemareg_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSchemareg(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Schemareg Suite")
}
