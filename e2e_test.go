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

func TestE2EMQTTExporter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("[E2E] 1. Finding an available port...")
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to find an available port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	listenAddr := fmt.Sprintf(":%d", port)
	log.Printf("[E2E] Using listen address: %s", listenAddr)

	log.Println("[E2E] 2. Starting mqtt_topic_exporter...")
	cmd := exec.CommandContext(ctx, "go", "run", "main.go",
		"--mqtt.server=mqtt://mosquitto:1883",
		"--mqtt.topic=/e2e/test",
		fmt.Sprintf("--web.listen-address=%s", listenAddr))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start mqtt_topic_exporter: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			log.Println("[E2E] Killing mqtt_topic_exporter process group...")
			pgid, _ := syscall.Getpgid(cmd.Process.Pid)
			syscall.Kill(-pgid, syscall.SIGKILL)
			log.Println("[E2E] Waiting for mqtt_topic_exporter to exit...")
			cmd.Wait()
			log.Println("[E2E] mqtt_topic_exporter process group exited")
		}
	})

	metricsURL := fmt.Sprintf("http://localhost:%d/metrics", port)

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
		t.Fatal("mqtt_topic_exporter did not start in time")
	}

	log.Println("[E2E] 4. Publishing a test message using Go MQTT client...")
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
		TopicName: []byte("/e2e/test"),
		Message:   []byte("42.5"),
		QoS:       0,
	}); err != nil {
		t.Fatalf("failed to publish test message: %v", err)
	}
	time.Sleep(1 * time.Second) // Wait for the message to be processed
	mqttCli.Disconnect()

	log.Println("[E2E] 5. Checking metrics...")
	found := false
	for i := 0; i < 10; i++ {
		resp, err := http.Get(metricsURL)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if strings.Contains(string(body), "mqtt_topic{topic=\"/e2e/test\"} 42.5") {
			found = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !found {
		t.Fatal("Expected metric not found in /metrics output")
	}

	log.Println("[E2E] 6. E2E test passed")
}
