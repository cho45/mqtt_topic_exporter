// #!/usr/bin/env go run
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"
)

// https://github.com/prometheus/node_exporter/blob/master/node_exporter.go

var topicLastHandled = map[string]time.Time{}
var topicLastHandledMutex = new(sync.Mutex)

var namespace = "mqtt"

var mqttGauge *prometheus.GaugeVec

func init() {
	mqttGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: fmt.Sprintf("%s_topic", namespace),
		Help: "mqtt topic gauge",
	}, []string{"topic"})

	prometheus.MustRegister(version.NewCollector("mqtt_exporter"))
	prometheus.MustRegister(mqttGauge)
}

var (
	Version = "dev"
)

func runMQTTSubscriber(ctx context.Context, retainTime time.Duration, uri *url.URL, tlsConfig *tls.Config, mqttTopics []string, maxBackoffSec int) {
	username := uri.User.Username()
	password, _ := uri.User.Password()

	log.Printf("value retain: %d", retainTime)

	go func() {
		errChan := make(chan error)
		defer close(errChan)
		for {
			select {
			case <-ctx.Done():
				log.Printf("MQTT loop exiting due to shutdown signal.")
				return
			default:
			}
			log.Printf("Connecting %s with topic %s", uri.String(), strings.Join(mqttTopics, " "))

			cli := client.New(&client.Options{
				ErrorHandler: func(err error) {
					errChan <- err
				},
			})
			defer cli.Terminate()

			clientID := fmt.Sprintf("mqtt_topic_exporter_%08x", time.Now().UnixNano()&0xffffffff)
			log.Printf("Using ClientID: %s", clientID)

			backoff := 1 * time.Second
			maxBackoff := time.Duration(maxBackoffSec) * time.Second

			for {
				err := cli.Connect(&client.ConnectOptions{
					Network:   "tcp",
					TLSConfig: tlsConfig,
					Address:   uri.Host,
					UserName:  []byte(username),
					Password:  []byte(password),
					ClientID:  []byte(clientID),
				})
				if err != nil {
					log.Printf("Failed to connect to MQTT broker: %v. Retrying in %v...", err, backoff)
					time.Sleep(backoff)
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}
				break
			}

			for _, mqttTopic := range mqttTopics {
				log.Printf("Subscribe topic %s", mqttTopic)
				err := cli.Subscribe(&client.SubscribeOptions{
					SubReqs: []*client.SubReq{
						&client.SubReq{
							TopicFilter: []byte(mqttTopic),
							QoS:         mqtt.QoS0,
							Handler: func(topicName, message []byte) {
								topic := string(topicName)
								topicLastHandledMutex.Lock()
								topicLastHandled[topic] = time.Now()
								topicLastHandledMutex.Unlock()
								value, err := strconv.ParseFloat(string(message), 64)
								if err != nil {
									log.Printf("Failed to parse float from MQTT message on topic %s: %s (error: %v)", topic, string(message), err)
									return
								}
								mqttGauge.WithLabelValues(topic).Set(value)
								log.Printf("MQTT TOPIC %s => %f", topic, value)
							},
						},
					},
				})
				if err != nil {
					log.Printf("Failed to subscribe to topic %s: %v. Retrying in 3 seconds...", mqttTopic, err)
					time.Sleep(3 * time.Second)
					continue
				}
			}

			disconnected := <-errChan
			log.Printf("MQTT Client disconnected %v", disconnected)

			cli.Terminate()
			backoff = 1 * time.Second
			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Printf("Cleanup loop exiting due to shutdown signal.")
				return
			default:
			}
			time.Sleep(10 * time.Second)
			now := time.Now()
			topicLastHandledMutex.Lock()
			for topic, last := range topicLastHandled {
				duration := now.Sub(last)
				if duration > retainTime {
					mqttGauge.DeleteLabelValues(topic)
					delete(topicLastHandled, topic)
					log.Printf("Deleted old topic %s", topic)
				}
			}
			topicLastHandledMutex.Unlock()
		}
	}()
}

// HTTPサーバー起動処理を分離
func runHTTPServer(ctx context.Context, listenAddress, metricsPath string) {
	httpServer := &http.Server{Addr: listenAddress}
	http.Handle(metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<!DOCTYPE html>
		<title>MQTT Exporter</title>
		<h1>MQTT Exporter</h1>
		<p><a href="` + metricsPath + `">Metrics</a>
		`))
	})

	go func() {
		log.Printf("Listening on %s", listenAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("Shutting down HTTP server...")
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctxTimeout)
	log.Printf("Shutdown complete.")
}

func main() {
	var (
		listenAddress = flag.String("web.listen-address", ":9981", "Address on which to expose metrics and web interface.")
		metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
		retainTimeStr = flag.String("mqtt.retain-time", "1m", "Retain duration for a topic")
		mqttServerUri = flag.String("mqtt.server", "", "MQTT Server address URI mqtts://user:pass@host:port")
		mqttTopics    = flag.String("mqtt.topic", "", "Comma separated MQTT topics to watch")
		maxBackoffSec = flag.Int("mqtt.max-backoff", 60, "Maximum backoff duration for MQTT reconnect attempts (seconds)")
	)
	flag.Parse()

	if *mqttServerUri == "" || *mqttTopics == "" {
		log.Fatal("--mqtt.server and --mqtt.topic are required")
	}

	topics := strings.Split(*mqttTopics, ",")

	log.Printf("Starting mqtt_topic_exporter version=%s", Version)
	log.Printf("Build context")
	log.Printf("--- Configuration ---")
	log.Printf("Listen address: %s", *listenAddress)
	log.Printf("Metrics path: %s", *metricsPath)
	uri, err := parseMQTTUri(*mqttServerUri)
	if err != nil {
		log.Fatalf("invalid mqtt.server uri: %v", err)
	}
	maskedUri := *uri
	if maskedUri.User != nil {
		user := maskedUri.User.Username()
		if _, hasPw := maskedUri.User.Password(); hasPw {
			maskedUri.User = url.UserPassword(user, "...")
		} else {
			maskedUri.User = url.User(user)
		}
	}
	log.Printf("MQTT server URI: %s", maskedUri.String())
	log.Printf("MQTT topics: %s", strings.Join(topics, ", "))
	log.Printf("Retain time: %s", *retainTimeStr)
	log.Printf("Max reconnect backoff: %d seconds", *maxBackoffSec)
	log.Printf("----------------------")

	retainTime, err := time.ParseDuration(*retainTimeStr)
	if err != nil {
		log.Fatalf("specified %s is invalid", *retainTimeStr)
	}

	uri, err = parseMQTTUri(*mqttServerUri)
	if err != nil {
		log.Fatalf("invalid mqtt.server uri: %v", err)
	}
	var tlsConfig *tls.Config
	if uri.Scheme == "mqtts" {
		tlsConfig = &tls.Config{}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runMQTTSubscriber(ctx, retainTime, uri, tlsConfig, topics, *maxBackoffSec)
	runHTTPServer(ctx, *listenAddress, *metricsPath)
}

// parseMQTTUri parses mqtt[s]://user:pass@host:port 形式のURIを返す
func parseMQTTUri(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
