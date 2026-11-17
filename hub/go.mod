module agent-control-plane/hub

go 1.27.4

require (
	agent-control-plane/harness-sdk v0.0.0-00010101000000-000000000000
	github.com/go-chi/chi/v5 v5.2.3
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2
	golang.org/x/sys v0.50.0
	nhooyr.io/websocket v1.8.17
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/text v0.37.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.55.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
	modernc.org/sqlite v1.34.5 // indirect
)

replace agent-control-plane/harness-sdk => ../harness-sdk
