package main

import (
	"fmt"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// handler is a simple function that takes a string and does a ToUpper.
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Println("FUNCTION HANDLER", request.Path)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       strings.ToUpper(request.Path[1:]),
	}, nil
}

func main() {
	fmt.Println("STARTING FUNCTION")
	lambda.Start(handler)
}
