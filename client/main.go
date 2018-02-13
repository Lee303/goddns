package main

import (
	"bytes"
	"ddns/lib"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

type Poller struct {
	IPHost        string
	Host          string
	APIURL        string
	APIAuthKey    string
	RecordType    lib.DDNSRecordType
	LastIPAddress string
	Interval      time.Duration
}

var lastIPv4Address string
var lastIPv6Address string

func main() {
	config, err := LoadConfig("ddnsclient.yml")
	check(err)

	interval, err := time.ParseDuration(config.Interval)
	check(err)

	ipv4Poller := &Poller{
		IPHost:     config.IPHost,
		Host:       config.Host,
		APIURL:     config.API.URL,
		APIAuthKey: config.API.AuthKey,
		RecordType: lib.A,
		Interval:   interval,
	}

	ipv6Poller := &Poller{
		IPHost:     config.IPHost,
		Host:       config.Host,
		APIURL:     config.API.URL,
		APIAuthKey: config.API.AuthKey,
		RecordType: lib.AAAA,
		Interval:   interval,
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals)
	go func() {
		_ = <-signals
		log.Print("Terminating")
		os.Exit(0)
	}()

	go ipv4Poller.Start()
	go ipv6Poller.Start()

	select {}
}

func (p *Poller) Start() {
	for {
		newIPAddress, err := p.getIPAddress()
		if err != nil {
			log.Printf("failed to retrieve ip address: %s", err)
			return
		}

		if newIPAddress != p.LastIPAddress {

			log.Printf("updating DDNS host %s with new IP address %s", p.Host, newIPAddress)

			err = p.updateDDNSHost(newIPAddress)
			if err != nil {
				log.Printf("failed to update DDNS host: %s", err)
				return
			}

			log.Print("successfully updated DDNS host")

			p.LastIPAddress = newIPAddress
		}

		time.Sleep(p.Interval)
	}
}

func (p *Poller) updateDDNSHost(ipAddress string) error {

	payload := lib.DDNSRecordBody{
		AuthKey:    p.APIAuthKey,
		IPAddress:  ipAddress,
		RecordType: p.RecordType,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %s", err)
	}

	client := http.Client{Timeout: time.Second * 30}

	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/ddns/%s", p.APIURL, p.Host), bytes.NewBuffer(payloadJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %s", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("unsuccessful request: statuscode=%d", resp.StatusCode)
	}

	return nil
}

func (p *Poller) getIPAddress() (string, error) {
	var ipAddress string
	var err error
	switch p.RecordType {
	case lib.A:
		ipAddr, err := net.ResolveIPAddr("ip4", p.IPHost)
		if err != nil {
			return "", err
		}

		ipAddress = ipAddr.IP.String()
		break
	case lib.AAAA:
		ipAddr, err := net.ResolveIPAddr("ip6", p.IPHost)
		if err != nil {
			return "", err
		}
		ipAddress = fmt.Sprintf("[%s]", ipAddr.IP.String())
		break
	default:
		return "", errors.New("invalid DDNS record type")
	}

	client := http.Client{Timeout: time.Second * 30}
	req, err := http.NewRequest("GET", "http://"+ipAddress, nil)
	if err != nil {
		return "", err
	}
	req.Host = p.IPHost
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("non-200 response): %d", resp.StatusCode)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %s", err)

	}

	return strings.TrimSpace(string(bodyBytes)), nil
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}