package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	InfluxDb  InfluxDb  `json:"influxdb"`
	Bluetooth Bluetooth `json:"bluetooth"`
	Sensors   []Sensor  `json:"sensors"`
}

type MacAddress [6]byte

type Bluetooth struct {
	Controller string `json:"controller"`
}

type Sensor struct {
	Location   string     `json:"location"`
	MacAddress MacAddress `json:"address"`
}

type InfluxDb struct {
	Url    string `json:"url"`
	Token  string `json:"token"`
	Org    string `json:"org"`
	Bucket string `json:"bucket"`
}

func (n *MacAddress) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	mac, err := macStringToBytes(s)
	if err != nil {
		return err
	}
	*n = MacAddress(mac)
	return nil
}

func macStringToBytes(mac string) ([6]byte, error) {
	var bytes [6]byte
	_, err := fmt.Sscanf(mac, "%02x:%02x:%02x:%02x:%02x:%02x",
		&bytes[0], &bytes[1], &bytes[2], &bytes[3], &bytes[4], &bytes[5])
	return bytes, err
}

func LoadJsonConfig(filename string) (Config, error) {
	var config Config

	body, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return config, err
	}

	if err := json.Unmarshal(body, &config); err != nil {
		fmt.Println("Error decoding JSON from file:", err)
		return config, err
	}

	return config, nil
}
