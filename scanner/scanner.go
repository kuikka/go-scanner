package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"time"

	"github.com/InfluxCommunity/influxdb3-go/v2/influxdb3"

	"tinygo.org/x/bluetooth"
)

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

func write(client *influxdb3.Client, point DataPoint) {
	data := []any{point}
	err := client.WriteData(context.Background(), data)
	must("write data point", err)
}

var Sensors = []Sensor{}

var (
	// Command line flags
	configFile = flag.String("c", "", "config file")
	verbose    = flag.Bool("verbose", false, "verbose")
)

func main() {

	flag.Parse()

	config, err := LoadJsonConfig(*configFile)
	if err != nil {
		panic(err)
	}
	Sensors = config.Sensors

	fmt.Printf("Config: %+v\n", config)

	// Instantiate the database client.
	dbClient, err := influxdb3.New(influxdb3.ClientConfig{
		Host:         config.InfluxDb.Url,
		Token:        config.InfluxDb.Token,
		Database:     config.InfluxDb.Bucket,
		Organization: config.InfluxDb.Org,
	})
	must("connect to database", err)

	// Instantiate the BLE adapter.
	var controller = bluetooth.NewAdapter(config.Bluetooth.Controller)

	// Enable BLE interface.
	must("enable BLE stack", controller.Enable())

	// Start scanning.
	println("scanning...")
	err = controller.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		for _, value := range device.AdvertisementPayload.ManufacturerData() {
			if value.CompanyID == 1177 {
				parse_ruuvi_data(dbClient, value.Data)
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
		var TxPower = -40 + 2*(float64(rawData.PowerInfo&0x1f))

		for _, sensor := range Sensors {
			if sensor.MacAddress == rawData.MacAddress {
				if *verbose {
					fmt.Printf("Got data from %s\n", sensor.Location)
					fmt.Printf("  Temperature = %f degC\n", Temperature)
					fmt.Printf("  Humidity = %f %%\n", Humidity)
					fmt.Printf("  AtmospherePressure = %f kPa\n", AtmospherePressure)
					fmt.Printf("  BatteryVoltage = %f V\n", BatteryVoltage)
					fmt.Printf("  TxPower = %f dBm\n", TxPower)
					fmt.Printf("  RSSI = %f dBm\n", float64(rawData.PowerInfo&0x1f))
				}
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
