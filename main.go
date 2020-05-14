package main

import (
	"flag"
	"fmt"
	"net/http"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/lisa/k8s-webhook-framework/pkg/webhooks"
)

var log = logf.Log.WithName("handler")

var (
	listenAddress = flag.String("listen", "0.0.0.0", "listen address")
	listenPort    = flag.String("port", "443", "port to listen on")
)

func main() {
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(true))
	log.Info("HTTP server running at", "listen", fmt.Sprintf("%s:%s", *listenAddress, *listenPort))
	seen := make(map[string]bool)
	for name, hook := range webhooks.Webhooks {
		if seen[hook().GetURI()] {
			panic(fmt.Errorf("Duplicate webhook trying to lisen on %s", hook().GetURI()))
		}
		seen[name] = true
		log.Info("Listening", "webhookName", name, "URI", hook().GetURI())
		http.HandleFunc(hook().GetURI(), hook().HandleRequest)
	}

	log.Error(http.ListenAndServe(fmt.Sprintf("%s:%s", *listenAddress, *listenPort), nil), "Error serving")

}
