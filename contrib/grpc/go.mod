module github.com/madcok-co/unicorn/contrib/grpc

go 1.25.1

replace github.com/madcok-co/unicorn => ../../..

require (
	github.com/madcok-co/unicorn v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.77.0
)

require (
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)
