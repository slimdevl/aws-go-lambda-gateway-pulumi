package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Help function to generate an IAM policy
func generatePolicy(principalId, effect, resource string) events.APIGatewayCustomAuthorizerResponse {
	authResponse := events.APIGatewayCustomAuthorizerResponse{PrincipalID: principalId}

	if effect != "" && resource != "" {
		authResponse.PolicyDocument = events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   effect,
					Resource: []string{resource},
				},
			},
		}
	}

	// Optional output with custom properties of the String, Number or Boolean type.
	authResponse.Context = map[string]interface{}{
		"stringKey":  "stringval",
		"numberKey":  123,
		"booleanKey": true,
	}
	return authResponse
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getHeaders(event events.APIGatewayCustomAuthorizerRequestTypeRequest) *http.Header {
	headers := &http.Header{}
	for k, v := range event.Headers {
		headers.Add(k, v)
	}

	return headers
}

var (
	ErrInvalidToken = errors.New("Invalid token")
	ErrUnauthorized = errors.New("Unauthorized")
)

const (
	DefaultTokenType = ""
	APITokenType     = "api.token"
)

func IsValidTokenType(name string) bool {
	if _, ok := TokenTypes[strings.ToLower(name)]; ok {
		return true
	}

	return false
}

var TokenTypes = map[string]struct{}{
	DefaultTokenType: {},
	APITokenType:     {},
}

func handleRequest(ctx context.Context, event events.APIGatewayCustomAuthorizerRequestTypeRequest) (events.APIGatewayCustomAuthorizerResponse, error) {
	headers := getHeaders(event)
	parts := strings.Split(headers.Get("Authorization"), " ")
	if len(parts) != 2 {
		return events.APIGatewayCustomAuthorizerResponse{}, ErrUnauthorized
	}
	if len(parts) != 2 {
		return events.APIGatewayCustomAuthorizerResponse{}, ErrUnauthorized
	}
	rawData, _ := base64.StdEncoding.DecodeString(parts[1])
	authParts := strings.SplitN(string(rawData), ":", 2)
	if len(authParts) != 2 {
		return events.APIGatewayCustomAuthorizerResponse{}, ErrUnauthorized
	}
	authType := authParts[0]
	authToken := authParts[1]
	fmt.Println("authType", authType)
	fmt.Println("authToken", authToken)
	if !IsValidTokenType(authParts[0]) {
		return events.APIGatewayCustomAuthorizerResponse{}, ErrUnauthorized
	}

	switch strings.ToLower(authToken) {
	case "allow":
		return generatePolicy("user", "Allow", event.MethodArn), nil
	case "deny":
		return generatePolicy("user", "Deny", event.MethodArn), nil
	case "unauthorized":
		return events.APIGatewayCustomAuthorizerResponse{}, errors.New("Unauthorized") // Return a 401 Unauthorized response
	default:
		secret := getEnv("ACCESS_TOKEN", "fail")
		if secret != "fail" && authToken == secret {
			return generatePolicy("user", "Allow", event.MethodArn), nil
		}
		return events.APIGatewayCustomAuthorizerResponse{}, ErrInvalidToken
	}
}

func main() {
	fmt.Println("STARTING AUTH")
	lambda.Start(handleRequest)
}
