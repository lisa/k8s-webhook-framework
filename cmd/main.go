package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/lisa/k8s-webhook-framework/pkg/webhooks"
)

var log = logf.Log.WithName("handler")

var (
	listenAddress = flag.String("listen", "0.0.0.0", "listen address")
	listenPort    = flag.String("port", "5000", "port to listen on")

	useTLS  = flag.Bool("tls", false, "Use TLS? Must specify -tlskey, -tlscert, -cacert")
	tlsKey  = flag.String("tlskey", "", "TLS Key for TLS")
	tlsCert = flag.String("tlscert", "", "TLS Certificate")
	caCert  = flag.String("cacert", "", "CA Cert file")
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

	server := &http.Server{
		Addr: fmt.Sprintf("%s:%s", *listenAddress, *listenPort),
	}
	if *useTLS {
		cafile, err := ioutil.ReadFile(*caCert)
		if err != nil {
			log.Error(err, "Couldn't read CA cert file")
			os.Exit(1)
		}
		certpool := x509.NewCertPool()
		certpool.AppendCertsFromPEM(cafile)

		server.TLSConfig = &tls.Config{
			RootCAs: certpool,
		}
		log.Error(server.ListenAndServeTLS(*tlsCert, *tlsKey), "Error serving TLS")
	} else {
		log.Error(server.ListenAndServe(), "Error serving non-TLS connection")
	}

}
