package main

import (
	"github.com/lisa/k8s-webhook-framework/pkg/certinjector"
)

func main() {
	injector := certinjector.NewCertInjector()
	err := injector.Inject()
	if err != nil {
		panic(err)
	}
}
