APP_NAME = mqtt_topic_exporter
VERSION ?= dev
LDFLAGS = -X 'main.Version=$(VERSION)'

.PHONY: all build clean run test-mqtt-server stop-mqtt-server

all: build

build:
	go build -ldflags="$(LDFLAGS)" -o $(APP_NAME) main.go

run: build
	./$(APP_NAME) --mqtt.server=mqtt://mosquitto:1883 --mqtt.topic=/test/topic

test-mqtt-server:
	docker run --rm -v $(PWD)/mosquitto.conf:/mosquitto/config/mosquitto.conf -p 1883:1883 eclipse-mosquitto

clean:
	rm -f $(APP_NAME)

test:
	go test -tags=e2e -v e2e_test.go
