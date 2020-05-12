package main

import (
	"fmt"
	"net/http"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/lisa/k8s-webhook-framework/pkg/webhooks"
)

var log = logf.Log.WithName("handler")

const listenHostAndPort string = "0.0.0.0:8888"

func main() {
	logf.SetLogger(logf.ZapLogger(true))
	log.Info("HTTP server running at", "listen", listenHostAndPort)
	seen := make(map[string]bool)
	for name, hook := range webhooks.Webhooks {
		if seen[hook().GetURI()] {
			panic(fmt.Errorf("Duplicate webhook trying to lisen on %s", hook().GetURI()))
		}
		seen[name] = true
		log.Info("Listening", "webhookName", name, "URI", hook().GetURI())
		http.HandleFunc(hook().GetURI(), hook().HandleRequest)
	}

	log.Error(http.ListenAndServe(listenHostAndPort, nil), "Error serving")

}
