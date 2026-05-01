package services

import (
	"errors"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func CreateMQTTClient(broker, clientID, username, password string) mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetOnConnectHandler(func(_ mqtt.Client) {
		fmt.Printf("Connected to MQTT broker at %s\n", broker)
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		fmt.Printf("MQTT connection lost: %v\n", err)
	})

	return mqtt.NewClient(opts)
}

func ConnectMQTTClient(client mqtt.Client) error {
	if client == nil {
		return errors.New("mqtt client is nil")
	}
	if client.IsConnected() {
		return nil
	}

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}
	fmt.Println("Connected to MQTT broker")
	return nil
}

func PublishMQTTMessage(client mqtt.Client, topic string, payload []byte) error {
	if client == nil || !client.IsConnected() {
		return errors.New("mqtt client is not connected")
	}

	token := client.Publish(topic, 0, false, payload)
	if !token.WaitTimeout(10 * time.Second) {
		return errors.New("timed out waiting for mqtt publish to complete")
	}
	return token.Error()
}

func SubscribeMQTTTopic(client mqtt.Client, topic string, handler mqtt.MessageHandler) error {
	if client == nil || !client.IsConnected() {
		return errors.New("mqtt client is not connected")
	}

	token := client.Subscribe(topic, 0, handler)
	if !token.WaitTimeout(10 * time.Second) {
		return errors.New("timed out waiting for mqtt subscribe to complete")
	}
	return token.Error()
}

func DisconnectMQTTClient(client mqtt.Client) {
	if client == nil || !client.IsConnected() {
		return
	}

	client.Disconnect(250)
	fmt.Println("Disconnected from MQTT broker")
}
