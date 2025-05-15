package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	client "github.com/yosssi/gmq/mqtt/client"
)

func main() {
	flag.Parse()

	if flag.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "Usage: pub_test [topic] [value]\n")
		os.Exit(1)
	}
	topic := flag.Arg(0)
	value := flag.Arg(1)

	// Use mosquitto:1883 as broker address (fixed)
	addr := "mosquitto:1883"

	cli := client.New(&client.Options{})
	defer cli.Terminate()

	if err := cli.Connect(&client.ConnectOptions{
		Network:  "tcp",
		Address:  addr,
		ClientID: []byte(fmt.Sprintf("pub_test_%d", time.Now().UnixNano())),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}

	// Validate value is float (for exporter compatibility)
	if _, err := strconv.ParseFloat(value, 64); err != nil {
		fmt.Fprintf(os.Stderr, "Value must be a float: %v\n", err)
		os.Exit(1)
	}

	if err := cli.Publish(&client.PublishOptions{
		TopicName: []byte(topic),
		Message:   []byte(value),
		QoS:       0,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to publish: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Published %s to %s\n", value, topic)
	cli.Disconnect()
}
