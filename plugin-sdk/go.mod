module agent-control-plane/plugin-sdk

go 1.27.4

require agent-control-plane/harness-sdk v0.0.0

require (
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	golang.org/x/text v0.37.0 // indirect
	nhooyr.io/websocket v1.8.17 // indirect
)

replace agent-control-plane/harness-sdk => ../harness-sdk
