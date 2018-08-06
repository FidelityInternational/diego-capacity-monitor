package main

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"

	metricsLib "github.com/FidelityInternational/diego-capacity-monitor/metrics"
	webs "github.com/FidelityInternational/diego-capacity-monitor/web_server"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/cloudfoundry/noaa/consumer"
)

var messageMetrics map[string]metricsLib.MessageMetric
var cellMemory float64
var watermark int

func main() {
	c := &cfclient.Config{
		ApiAddress:        os.Getenv("CF_API_ENDPOINT"),
		Username:          os.Getenv("CF_USERNAME"),
		Password:          os.Getenv("CF_PASSWORD"),
		SkipSslValidation: true,
	}

	client, err := cfclient.NewClient(c)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	watermark := os.Getenv("WATERMARK")
	if watermark == "" {
		fmt.Println("No WATERMARK environment variable supplied, so will default to 1")
		watermark = "1"
	}

	cnsmr := consumer.New(client.Endpoint.DopplerEndpoint, &tls.Config{InsecureSkipVerify: true}, nil)
	cnsmr.SetDebugPrinter(consoleDebugPrinter{})

	authToken, err := client.GetToken()
	if err != nil {
		fmt.Println("Error occurred grabbing oauth token")
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Println("===== Streaming Firehose (will only succeed if you have admin credentials)")
	metrics := metricsLib.CreateMetrics()

	server := webs.CreateServer(metrics, &cellMemory, &watermark)

	router := server.Start()

	http.Handle("/", router)

	go func() {
		err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
		if err != nil {
			fmt.Println("ListenAndServe:", err)
		}
	}()

	firehoseSubscriptionID, err := newUUID()
	if err != nil {
		fmt.Println("Error occurred generating subscription ID")
		fmt.Println(err.Error())
		os.Exit(1)
	}

	msgChan, errorChan := cnsmr.FilteredFirehose(firehoseSubscriptionID, authToken, consumer.Metrics)
	go func() {
		for err := range errorChan {
			fmt.Fprintf(os.Stderr, "%v\n", err.Error())
			os.Exit(1)
		}
	}()

	cellMemory = 0

	go func() {
		ticker := time.NewTicker(metrics.StaleDuration)

		for range ticker.C {
			metrics.ClearStaleMetrics()
		}
	}()

	for msg := range msgChan {
		if cellMemory == 0 {
			match, _ := regexp.MatchString(".*diego[_-]cell.*CapacityTotalMemory.*", msg.String())
			if match {
				cellMemory = msg.ValueMetric.GetValue()
				fmt.Printf("Setting the max memory to %v\n", cellMemory)
			}
		}

		match, err := regexp.MatchString(".*diego[_-]cell.*CapacityRemainingMemory.*", msg.String())
		if err != nil {
			fmt.Println("An error occurred matching diego cells, skipping to next message")
			fmt.Println(err.Error())
			continue
		}
		if match {
			metrics.Set(*msg.Index, metricsLib.MessageMetric{Memory: msg.ValueMetric.GetValue(), Timestamp: *msg.Timestamp})
			fmt.Printf("Index: %v, Value: %v, Timeout: %v\n", *msg.Index, msg.ValueMetric.GetValue(), *msg.Timestamp)
		}
	}
}

type consoleDebugPrinter struct{}

func (c consoleDebugPrinter) Print(title, dump string) {
	println(title)
	println(dump)
}

func newUUID() (string, error) {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return "", err
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}
