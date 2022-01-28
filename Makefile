build::
	GOOS=linux GOARCH=amd64 go build -o ./function/handler ./function/main.go
	zip -j ./function.zip ./function/handler
	GOOS=linux GOARCH=amd64 go build -o ./authorizer/handler ./authorizer/main.go
	zip -j ./authorizer.zip ./authorizer/handler
