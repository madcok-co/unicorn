module github.com/madcok-co/unicorn/contrib/broker/mqtt

go 1.25.1

replace github.com/madcok-co/unicorn => ../../..

require (
	github.com/eclipse/paho.mqtt.golang v1.5.0
	github.com/madcok-co/unicorn v0.0.0-00010101000000-000000000000
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
)
