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
