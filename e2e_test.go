//go:build e2e
// +build e2e

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	importedClient "github.com/yosssi/gmq/mqtt/client"
)

type targetProc struct {
	cmd        *exec.Cmd
	metricsURL string
	port       int
}

func emptyPort(_ interface{}) int {
	ln, _ := net.Listen("tcp", ":0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func startTarget(ctx context.Context) (*targetProc, func(), error) {
	port := emptyPort(nil) // tは使わない
	listenAddr := fmt.Sprintf(":%d", port)
	log.Printf("[E2E] Using listen address: %s", listenAddr)

	cmd := exec.CommandContext(ctx, "go", "run", "main.go",
		"--mqtt.server=mqtt://mosquitto:1883",
		"--mqtt.topic=/e2e/test/#",
		fmt.Sprintf("--web.listen-address=%s", listenAddr))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start mqtt_topic_exporter: %w", err)
	}
	metricsURL := fmt.Sprintf("http://localhost:%d/metrics", port)

	cleanup := func() {
		if cmd.Process != nil {
			log.Println("[E2E] Killing mqtt_topic_exporter process group...")
			pgid, _ := syscall.Getpgid(cmd.Process.Pid)
			syscall.Kill(-pgid, syscall.SIGKILL)
			log.Println("[E2E] Waiting for mqtt_topic_exporter to exit...")
			cmd.Wait()
			log.Println("[E2E] mqtt_topic_exporter process group exited")
		}
	}

	log.Println("[E2E] 3. Waiting for exporter to be ready...")
	ready := false
	for i := 0; i < 10; i++ {
		resp, err := http.Get(metricsURL)
		if err == nil && resp.StatusCode == 200 {
			ready = true
			resp.Body.Close()
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !ready {
		cleanup()
		return nil, nil, fmt.Errorf("mqtt_topic_exporter did not start in time")
	}

	return &targetProc{cmd: cmd, metricsURL: metricsURL, port: port}, cleanup, nil
}

func mqttPublish(t *testing.T, topic, value string) {
	log.Printf("[E2E] Publishing to topic %s with value %s", topic, value)
	mqttCli := importedClient.New(&importedClient.Options{})
	defer mqttCli.Terminate()
	if err := mqttCli.Connect(&importedClient.ConnectOptions{
		Network:  "tcp",
		Address:  "mosquitto:1883",
		ClientID: []byte("e2e_test_pub"),
	}); err != nil {
		t.Fatalf("failed to connect to MQTT broker for publish: %v", err)
	}
	if err := mqttCli.Publish(&importedClient.PublishOptions{
		TopicName: []byte(topic),
		Message:   []byte(value),
		QoS:       0,
	}); err != nil {
		t.Fatalf("failed to publish test message: %v", err)
	}
	time.Sleep(1 * time.Second) // Wait for the message to be processed
	mqttCli.Disconnect()
}

func checkMetrics(t *testing.T, metricsURL, expected string) {
	log.Printf("[E2E] Checking /metrics for expected output: %s", expected)
	var lastBody string
	for i := 0; i < 5; i++ {
		resp, err := http.Get(metricsURL)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		lastBody = string(body)
		if strings.Contains(lastBody, expected) {
			return
		}
		time.Sleep(1 * time.Second)
	}
	// /metrics のうち mqtt_topic を含む行を抽出
	var lines []string
	for _, line := range strings.Split(lastBody, "\n") {
		if strings.Contains(line, "mqtt_topic") {
			lines = append(lines, line)
		}
	}
	t.Fatalf("Expected metric not found in /metrics output: %s\n--- mqtt_topic lines ---\n%s", expected, strings.Join(lines, "\n"))
}

func TestE2EMQTTExporter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	target, cleanup, err := startTarget(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	mqttPublish(t, "/e2e/test/0", "42.5")
	mqttPublish(t, "/e2e/test/1", "52.5")
	checkMetrics(t, target.metricsURL, `mqtt_topic{topic="/e2e/test/0"} 42.5`)
	checkMetrics(t, target.metricsURL, `mqtt_topic{topic="/e2e/test/1"} 52.5`)
	mqttPublish(t, "/e2e/test/0", "100.1")
	checkMetrics(t, target.metricsURL, `mqtt_topic{topic="/e2e/test/0"} 100.1`)
}
