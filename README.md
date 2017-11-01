# GitHub Webhook -> NATS Streaming

This is an HTTP server that exposes an endpoint that can be registered as a GitHub webhook URL. All events published by GitHub will be published onto a corresponding NATS Streaming channel.

The utility is a single ingest point for all events from GitHub, durability, and to support multiple consumers of these events.

## Install

*Platform binaries and Docker image coming soon..*

```
go get -u github.com/bruth/github-webhook-nats-streaming
```

## Usage

```
$ github-webhook-nats-streaming -h
Usage of github-webhook-nats-streaming:
  -github.secret string
    	GitHub secret.
  -http.addr string
    	HTTP bind address. (default "localhost:8080")
  -http.tls.cert string
    	HTTP TLS cert file.
  -http.tls.key string
    	HTTP TLS key file.
  -nats.addr string
    	NATS address. (default "nats://localhost:4222")
  -nats.tls.cert string
    	NATS TLS cert file.
  -nats.tls.key string
    	NATS TLS key file.
  -stan.channel string
    	STAN channel template. (default "github.events")
  -stan.client string
    	STAN client ID. (default "github-webhook")
  -stan.cluster string
    	STAN cluster ID. (default "test-cluster")
```

### -stan.channel

The specified channel determines the NATS Streaming channel events are published to. This value is *templated* meaning, it supports a few variables that can be specified in the string and are resolved every time an event comes in.

The default is a static string (`github.events`) that doesn't utilize any variables (and static string will work just fine). The variables available are:

- `Owner` - the username of the user or name of the organization that owns the repo the event was published from
- `Repo` - the name of the repo the event came from
- `Event` - the [event type](https://developer.github.com/webhooks/#events)

For example, you can use all of them to route every event to a unique channel specific to the owner, repo, and event type. Below is a valid channel option.

```
-stan.channel "github.events.{{.Owner}}.{{.Repo}}.{{.Event}}"
```

*The above uses Go's template syntax to access a field on the passed template struct.*

Given an `watch` event on the repo `nats-streaming-server` owned by `nats-io`, the event would be published to the `github.events.nats-io.nats-streaming-server.watch` channel.
