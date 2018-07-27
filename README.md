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
