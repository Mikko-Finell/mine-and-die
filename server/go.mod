module mine-and-die/server

go 1.24.3

require github.com/gorilla/websocket v1.5.1

replace github.com/gorilla/websocket => ./internal/stubs/websocket
