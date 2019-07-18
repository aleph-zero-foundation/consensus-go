package urn_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestUrn(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Urn Suite")
}
