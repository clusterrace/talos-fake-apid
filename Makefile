fetch-certs:
	scp 192.168.0.23:/home/ubuntu/talos-fake-apid/server.crt ./
	scp 192.168.0.23:/home/ubuntu/talos-fake-apid/server.key ./

protoc:
	rm -rf proto/v1alpha1/*.go
	protoc --proto_path=proto --go-grpc_out=paths=source_relative:./proto --go_out=paths=source_relative:./proto  proto/v1alpha1/*