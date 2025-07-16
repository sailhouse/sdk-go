package sailhouse_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSailhouse(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sailhouse SDK Suite")
}