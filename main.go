package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/nats-io/go-nats"
	"github.com/nats-io/go-nats-streaming"
)

func verifyGithubSignature(sig, secret string, b []byte) bool {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(b)
	exp := fmt.Sprintf("sha1=%x", mac.Sum(nil))
	return hmac.Equal([]byte(exp), []byte(sig))
}

type webhookEvent struct {
	Repository struct {
		Name  string
		Owner struct {
			Login string
		}
	}
}

type pingEvent struct {
	Zen string
}

type channelTmplVars struct {
	Owner string
	Repo  string
	Event string
}

func main() {
	var (
		natsAddr    string
		natsTlsKey  string
		natsTlsCert string

		stanCluster     string
		stanClient      string
		stanChannelTmpl string

		httpAddr    string
		httpTlsKey  string
		httpTlsCert string

		githubSecret string
	)

	flag.StringVar(&natsAddr, "nats.addr", "nats://localhost:4222", "NATS address.")
	flag.StringVar(&natsTlsKey, "nats.tls.key", "", "NATS TLS key file.")
	flag.StringVar(&natsTlsCert, "nats.tls.cert", "", "NATS TLS cert file.")

	flag.StringVar(&stanCluster, "stan.cluster", "test-cluster", "STAN cluster ID.")
	flag.StringVar(&stanClient, "stan.client", "github-webhook", "STAN client ID.")
	flag.StringVar(&stanChannelTmpl, "stan.channel", "github.events", "STAN channel template.")

	flag.StringVar(&httpAddr, "http.addr", "localhost:8080", "HTTP bind address.")
	flag.StringVar(&httpTlsKey, "http.tls.key", "", "HTTP TLS key file.")
	flag.StringVar(&httpTlsCert, "http.tls.cert", "", "HTTP TLS cert file.")

	flag.StringVar(&githubSecret, "github.secret", "", "GitHub secret.")

	flag.Parse()

	channelTmpl, err := template.New("channel").Parse(stanChannelTmpl)
	if err != nil {
		log.Fatalf("bad channel template: %s", err)
	}

	var opts []nats.Option
	if natsTlsKey != "" {
		opts = append(opts, nats.ClientCert(natsTlsCert, natsTlsKey))
	}

	nc, err := nats.Connect(natsAddr, opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	sc, err := stan.Connect(
		stanCluster,
		stanClient,
		stan.NatsConn(nc),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sc.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only POST requests.
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Read body for secret verification and/or sending to NATS Streaming.
		body, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			log.Print("error reading body: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		sig := r.Header.Get("x-hub-signature")

		if githubSecret != "" {
			// Signature expected.
			if sig == "" {
				log.Print("signature not present")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Verify signature.
			if !verifyGithubSignature(sig, githubSecret, body) {
				log.Print("signature did not match")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Signature present, but no secret.
		} else if sig != "" {
			log.Print("secret required/expected")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Ignore ping events.
		eventType := r.Header.Get("x-github-event")
		if eventType == "ping" {
			return
		}

		// Partial unmarshal to extract repo info.
		var evt webhookEvent
		if err := json.Unmarshal(body, &evt); err != nil {
			log.Printf("could not parse event: %s", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		var b bytes.Buffer
		err = channelTmpl.Execute(&b, &channelTmplVars{
			Owner: evt.Repository.Owner.Login,
			Repo:  evt.Repository.Name,
			Event: eventType,
		})
		if err != nil {
			log.Printf("template error: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := sc.Publish(b.String(), body); err != nil {
			log.Printf("publish error: %s", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	})

	if httpTlsKey != "" {
		http.ListenAndServeTLS(httpAddr, httpTlsCert, httpTlsKey, nil)
	} else {
		http.ListenAndServe(httpAddr, nil)
	}
}
