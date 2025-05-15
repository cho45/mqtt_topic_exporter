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

	"log"

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

func main() {
	var (
		listenAddress = flag.String("web.listen-address", ":9981", "Address on which to expose metrics and web interface.")
		metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
		retainTimeStr = flag.String("mqtt.retain-time", "1m", "Retain duration for a topic")
		mqttServerUri = flag.String("mqtt.server", "", "MQTT Server address URI mqtts://user:pass@host:port")
		mqttTopics    = flag.String("mqtt.topic", "", "Comma separated MQTT topics to watch")
	)
	flag.Parse()

	if *mqttServerUri == "" || *mqttTopics == "" {
		log.Fatal("--mqtt.server and --mqtt.topic are required")
	}

	topics := strings.Split(*mqttTopics, ",")

	log.Printf("Starting mqtt_topic_exporter version=%s", Version)
	log.Printf("Build context")

	// parse retain time
	retainTime, err := time.ParseDuration(*retainTimeStr)
	if err != nil {
		log.Fatalf("specified %s is invalid", *retainTimeStr)
	}

	// parse uri
	var tlsConfig *tls.Config
	uri, err := parseMQTTUri(*mqttServerUri)
	if err != nil {
		log.Fatalf("invalid mqtt.server uri: %v", err)
	}
	if uri.Scheme == "mqtts" {
		tlsConfig = &tls.Config{}
	}
	username := uri.User.Username()
	password, _ := uri.User.Password()

	log.Printf("value ratain: %d", retainTime)

	go func() {
		for {
			log.Printf("Connecting %s with topic %s", uri.String(), strings.Join(topics, " "))

			errChan := make(chan error)
			defer close(errChan)

			cli := client.New(&client.Options{
				ErrorHandler: func(err error) {
					errChan <- err
				},
			})
			defer cli.Terminate()

			err = cli.Connect(&client.ConnectOptions{
				Network:   "tcp",
				TLSConfig: tlsConfig,
				Address:   uri.Host,
				UserName:  []byte(username),
				Password:  []byte(password),
				ClientID:  []byte("client_id"),
			})
			if err != nil {
				log.Fatal(err)
			}

			// Subscribe to topics.
			for _, mqttTopic := range topics {
				log.Printf("Subscribe topic %s", mqttTopic)
				err := cli.Subscribe(&client.SubscribeOptions{
					SubReqs: []*client.SubReq{
						&client.SubReq{
							TopicFilter: []byte(mqttTopic),
							QoS:         mqtt.QoS0,
							Handler: func(topicName, message []byte) {
								// mqtt_topic{topic="/foo/bar"} value
								topic := string(topicName)
								topicLastHandledMutex.Lock()
								topicLastHandled[topic] = time.Now()
								topicLastHandledMutex.Unlock()
								value, _ := strconv.ParseFloat(string(message), 64)
								mqttGauge.WithLabelValues(topic).Set(value)
								log.Printf("MQTT TOPIC %s => %f", topic, value)
							},
						},
					},
				})
				if err != nil {
					log.Fatal(err)
				}
			}

			disconnected := <-errChan
			log.Printf("MQTT Client disconnected %v", disconnected)

			cli.Terminate()

			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		// Cleanup
		for {
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

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<!DOCTYPE html>
		<title>MQTT Exporter</title>
		<h1>MQTT Exporter</h1>
		<p><a href="` + *metricsPath + `">Metrics</a>
		`))
	})

	log.Printf("Listening on %s", *listenAddress)
	err = http.ListenAndServe(*listenAddress, nil)
	if err != nil {
		log.Fatal(err)
	}
}

// parseMQTTUri parses mqtt[s]://user:pass@host:port 形式のURIを返す
func parseMQTTUri(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
