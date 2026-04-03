module lostandfound-plugin

go 1.24.0

require github.com/superbot/wasmplugin v0.0.0

require (
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
)

replace github.com/superbot/wasmplugin => ../../sdk/go-plugin
