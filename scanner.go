package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/InfluxCommunity/influxdb3-go/v2/influxdb3"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

type DataPoint struct {
	Bucket             string    `lp:"measurement"`
	Location           string    `lp:"tag,location"`
	Temperature        float64   `lp:"field,temperature"`
	Humidity           float64   `lp:"field,humidity"`
	AtmospherePressure float64   `lp:"field,atmosphere_pressure"`
	RSSI               float64   `lp:"field,rssi"`
	BatteryVoltage     float64   `lp:"field,battery_voltage"`
	Time               time.Time `lp:"timestamp"`
}

type Sensor struct {
	Location   string
	MacAddress [6]byte
}

var Sensors = []Sensor {
	{"Family room", [6]byte{0xE0, 0xE7, 0xCD, 0x59, 0x5D, 0x74}},
	{"Office", [6]byte{0xD4, 0x7A, 0xAA, 0xC9, 0x5D, 0xD6}},
	{"Master bedroom", [6]byte{0xE6, 0x99, 0x94, 0x26, 0xC5, 0xC3}},
	{"Garage", [6]byte{0xD7, 0x44, 0x78, 0x0F, 0xA5, 0x65}},
	{"Kitchen", [6]byte{0xF6, 0x8C, 0xF2, 0x8D, 0x6E, 0xA3}},
}

func write(client *influxdb3.Client, point DataPoint) {
	data := []any{point}
	err := client.WriteData(context.Background(), data)
	must("write data point", err)
}

func main() {
	// Instantiate the client.
	client, err := influxdb3.New(influxdb3.ClientConfig{
		Host:         "http://influxdb:30115/",
		Token:        "_BpHKWqFu03oSWHcxSUyM723DC7ZUoLubaAbp8wkZFEk_FDTF23hNpfrAyPg0I_yinLcDJSfYXrK97RdpL4qRA==",
		Database:     "Sensor data",
		Organization: "Loon's Nest",
	})
	must("connect to database", err)

	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	// Start scanning.
	println("scanning...")
	err = adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		// println("found device:", device.Address.String(), device.RSSI, device.LocalName())
		for _, value := range device.AdvertisementPayload.ManufacturerData() {
			if value.CompanyID == 1177 {
				// fmt.Printf("%d\n", value.CompanyID)
				// fmt.Printf("%x\n", value.Data)
				parse_ruuvi_data(client, value.Data)
			}
		}
	})
	must("start scan", err)
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}

func parse_ruuvi_data(client *influxdb3.Client, data []byte) {
	/**
	  Parse Ruuvi advertising formats
	  see https://github.com/ruuvi/ruuvi-sensor-protocols/blob/master/broadcast_formats.md
	  and https://github.com/ruuvi/ruuvi-sensor-protocols/blob/master/dataformat_05.md
	  for details
	*/
	type RuuviRawV2Data struct {
		Temperature            int16
		Humidity               uint16
		AtmospherePressure     uint16
		AccelerationX          int16
		AccelerationY          int16
		AccelerationZ          int16
		PowerInfo              uint16
		MovementCounter        uint8
		MovementSequenceNumber uint16
		MacAddress             [6]byte
	}

	var packet_format = data[0]
	var rawData RuuviRawV2Data

	if packet_format == 5 {
		var reader = bytes.NewReader(data[1:])

		binary.Read(reader, binary.BigEndian, &rawData.Temperature)
		binary.Read(reader, binary.BigEndian, &rawData.Humidity)
		binary.Read(reader, binary.BigEndian, &rawData.AtmospherePressure)
		binary.Read(reader, binary.BigEndian, &rawData.AccelerationX)
		binary.Read(reader, binary.BigEndian, &rawData.AccelerationY)
		binary.Read(reader, binary.BigEndian, &rawData.AccelerationZ)
		binary.Read(reader, binary.BigEndian, &rawData.PowerInfo)
		binary.Read(reader, binary.BigEndian, &rawData.MovementCounter)
		binary.Read(reader, binary.BigEndian, &rawData.MovementSequenceNumber)
		binary.Read(reader, binary.BigEndian, &rawData.MacAddress)

		var Temperature = float64(rawData.Temperature) * 0.005
		var Humidity = float64(rawData.Humidity) * 0.0025
		var AtmospherePressure = (float64(rawData.AtmospherePressure) + float64(50000)) / 1000.0

		var BatteryVoltage = (float64(rawData.PowerInfo>>5) + float64(1600)) / 1000.0
		// var TxPower = -40 + 2*(float64(rawData.PowerInfo&0x1f))

		// fmt.Printf("Temperature = %f degC\n", Temperature)
		// fmt.Printf("Humidity = %f %%\n", Humidity)
		// fmt.Printf("AtmospherePressure = %f kPa\n", AtmospherePressure)

		// fmt.Printf("BatteryVoltage = %f V\n", BatteryVoltage)
		// fmt.Printf("TxPower = %f dBm\n", TxPower)

		// fmt.Printf("MAC %x\n", rawData.MacAddress)

		for _, sensor := range Sensors {
			if sensor.MacAddress == rawData.MacAddress {
				fmt.Printf("Got data from %s\n", sensor.Location)
				dp := DataPoint{
					Bucket:             "Sensor data",
					Location:           sensor.Location,
					Temperature:        Temperature,
					Humidity:           Humidity,
					AtmospherePressure: AtmospherePressure,
					BatteryVoltage:     BatteryVoltage,
				}
				write(client, dp)
			}
		}
	}
}
