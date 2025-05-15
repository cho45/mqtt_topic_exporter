mqtt\_topic\_exporter
==================

Subscribe MQTT topic and export them to prometheus.

```
usage: mqtt_topic_exporter --mqtt.server=MQTT.SERVER --mqtt.topic=MQTT.TOPIC [<flags>]

Flags: 
  -h, --help                     Show context-sensitive help (also try --help-long and --help-man).
      --web.listen-address=":9981"  
                                 Address on which to expose metrics and web interface.
      --web.telemetry-path="/metrics"  
                                 Path under which to expose metrics.
      --mqtt.retain-time="1m"    Retain duration for a topic
      --mqtt.server=MQTT.SERVER  MQTT Server address URI mqtts://user:pass@host:port
      --mqtt.topic=MQTT.TOPIC    Watch MQTT topic
      --log.level="info"         Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]
      --log.format="logger:stderr"  
                                 Set the log target and format. Example: "logger:syslog?appname=bob&local=7" or "logger:stdout?json=true"
      --version                  Show application version.
```

Sample output:
```
curl -s http://127.0.0.1:9981/metrics | grep mqtt
# HELP mqtt_exporter_build_info A metric with a constant '1' value labeled by version, revision, branch, and goversion from which mqtt_exporter was built.
# TYPE mqtt_exporter_build_info gauge
mqtt_exporter_build_info{branch="",goversion="go1.10.3",revision="",version=""} 1
# HELP mqtt_topic mqtt topic gauge
# TYPE mqtt_topic gauge
mqtt_topic{topic="/home/sensor/co2"} 462
mqtt_topic{topic="/home/sensor/temp"} 31.171875
```

## Development with Dev Container

This repository supports [VS Code Dev Containers](https://code.visualstudio.com/docs/devcontainers/containers).

### How to start the Dev Container
1. Open this repository in VS Code.
2. Open the command palette (`F1` or `Ctrl+Shift+P`) and select `Dev Containers: Reopen in Container`.

This will automatically build and start the development container environment.

---

## Build Instructions

```sh
make build
```
Or
```sh
go build -o mqtt_topic_exporter main.go
```

---

## Debugging and Testing

### 1. Start the MQTT broker

```sh
docker-compose up -d mosquitto
```

### 2. Run the application

```sh
make run
```
Or
```sh
./mqtt_topic_exporter --mqtt.server=mqtt://mosquitto:1883 --mqtt.topic=/test/topic
```

### 3. Check Prometheus metrics

In another terminal, run:

```sh
curl -s http://localhost:9981/metrics | grep mqtt
```

---

## Testing MQTT Publish (pubtest.go)

A sample Go program `pubtest.go` is included for publishing test messages to the MQTT broker.

### Usage

Run the publisher directly with `go run`:
```sh
 go run pubtest.go --topic /test/topic --value 42.5
```

This will publish a test message to the MQTT broker at `mqtt://mosquitto:1883`.

You can modify the command line arguments to change topics, payloads, or broker address as needed.

---

### Notes
- The MQTT broker configuration file is located at `.devcontainer/mosquitto.conf`.
- From inside the Dev Container, you can connect to the broker using `mqtt://mosquitto:1883`.
