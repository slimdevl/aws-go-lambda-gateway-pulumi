package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/apigateway"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		account, err := aws.GetCallerIdentity(ctx)
		if err != nil {
			return err
		}

		region, err := aws.GetRegion(ctx, &aws.GetRegionArgs{})
		if err != nil {
			return err
		}

		// Create an IAM role.
		role, err := iam.NewRole(ctx, "task-exec-role", &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
				"Version": "2012-10-17",
				"Statement": [{
					"Sid": "",
					"Effect": "Allow",
					"Principal": {
						"Service": "lambda.amazonaws.com"
					},
					"Action": "sts:AssumeRole"
				}]
			}`),
		})
		if err != nil {
			return err
		}

		// Attach a policy to allow writing logs to CloudWatch
		logPolicy, err := iam.NewRolePolicy(ctx, "lambda-log-policy",
			&iam.RolePolicyArgs{
				Role: role.Name,
				Policy: pulumi.String(`{
                "Version": "2012-10-17",
                "Statement": [{
                    "Effect": "Allow",
                    "Action": [
                        "logs:CreateLogGroup",
                        "logs:CreateLogStream",
                        "logs:PutLogEvents"
                    ],
                    "Resource": "arn:aws:logs:*:*:*"
                }]
            }`),
			},
			pulumi.DependsOn([]pulumi.Resource{role}),
		)
		if err != nil {
			return err
		}

		// Set arguments for constructing the function resource.
		args := &lambda.FunctionArgs{
			Handler: pulumi.String("handler"),
			Role:    role.Arn,
			Runtime: pulumi.String("go1.x"),
			Code:    pulumi.NewFileArchive("./function.zip"),
		}

		// Create the lambda using the args.
		function, err := lambda.NewFunction(
			ctx,
			"basicLambda",
			args,
			pulumi.DependsOn([]pulumi.Resource{logPolicy}),
		)
		if err != nil {
			return err
		}
		secret, err := apigateway.NewApiKey(ctx, "key", nil)
		if err != nil {
			return err
		}
		ctx.Export("key", pulumi.Sprintf("%s", secret.Value))
		// Set arguments for constructing the function resource.
		authFuncArgs := &lambda.FunctionArgs{
			Handler: pulumi.String("handler"),
			Role:    role.Arn,
			Runtime: pulumi.String("go1.x"),
			Code:    pulumi.NewFileArchive("./authorizer.zip"),
			Environment: &lambda.FunctionEnvironmentArgs{
				Variables: pulumi.StringMap{
					"ACCESS_TOKEN": secret.Value,
				},
			},
		}
		// Create the lambda using the args.
		authFunction, err := lambda.NewFunction(
			ctx,
			"authFunction",
			authFuncArgs,
			pulumi.DependsOn([]pulumi.Resource{logPolicy, secret}),
		)
		if err != nil {
			return err
		}

		// Create a new API Gateway.
		gateway, err := apigateway.NewRestApi(ctx, "UpperCaseGateway",
			&apigateway.RestApiArgs{
				Name:        pulumi.String("UpperCaseGateway"),
				Description: pulumi.String("An API Gateway for the UpperCase function"),
				Policy: pulumi.String(`{
                  "Version": "2012-10-17",
                  "Statement": [
                    {
                      "Action": "sts:AssumeRole",
                      "Principal": {
                        "Service": "lambda.amazonaws.com"
                      },
                      "Effect": "Allow",
                      "Sid": ""
                    },
                    {
                        "Action": "execute-api:Invoke",
                        "Resource": "*",
                        "Principal": "*",
                        "Effect": "Allow",
                        "Sid": ""
                    }
                  ]
                }`),
			},
		)
		if err != nil {
			return err
		}

		// Add a resource to the API Gateway.
		// This makes the API Gateway accept requests on "/{message}".
		apiresource, err := apigateway.NewResource(ctx, "UpperAPI",
			&apigateway.ResourceArgs{
				RestApi:  gateway.ID(),
				PathPart: pulumi.String("{proxy+}"),
				ParentId: gateway.RootResourceId,
			},
			pulumi.DependsOn([]pulumi.Resource{gateway}),
		)
		if err != nil {
			return err
		}
		// create an authorizor
		authorizer, err := apigateway.NewAuthorizer(ctx,
			"authorizer",
			&apigateway.AuthorizerArgs{
				AuthorizerUri:                authFunction.InvokeArn,
				AuthorizerResultTtlInSeconds: pulumi.Int(0),
				IdentitySource:               pulumi.String("method.request.header.Authorization"),
				Name:                         pulumi.String("authorizer"),
				RestApi:                      gateway.ID(),
				Type:                         pulumi.String("REQUEST"),
			},
			pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, function, authFunction}),
		)
		if err != nil {
			return err
		}
		// Add a method to the API Gateway.
		method, err := apigateway.NewMethod(ctx, "AnyMethod",
			&apigateway.MethodArgs{
				HttpMethod:     pulumi.String("ANY"),
				Authorization:  pulumi.String("CUSTOM"),
				ApiKeyRequired: pulumi.BoolPtr(false),
				RestApi:        gateway.ID(),
				ResourceId:     apiresource.ID(),
				AuthorizerId:   authorizer.ID(),
			},
			pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, authorizer, function}),
		)
		if err != nil {
			return err
		}

		// Add an integration to the API Gateway.
		// This makes communication between the API Gateway and the Lambda function work
		integration, err := apigateway.NewIntegration(ctx, "LambdaIntegration",
			&apigateway.IntegrationArgs{
				HttpMethod:            pulumi.String("ANY"),
				IntegrationHttpMethod: pulumi.String("POST"),
				ResourceId:            apiresource.ID(),
				RestApi:               gateway.ID(),
				Type:                  pulumi.String("AWS_PROXY"),
				Uri:                   function.InvokeArn,
			},
			pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, authorizer, function, authFunction, method}),
		)
		if err != nil {
			return err
		}
		////////////////////////////////////////////////////////////////////////////////////////////////////////////////
		// Setup CORS configuration for API Gateway
		// Mimic of https://github.com/mewa/terraform-aws-apigateway-cors
		// INPUTS:
		// - aws_api_gateway_rest_api == gateway
		// - aws_api_gateway_resource == apiresource
		// methods = ["GET", "POST", ...]
		corsmethod, err := apigateway.NewMethod(ctx, "CORSMethod",
			&apigateway.MethodArgs{
				HttpMethod:    pulumi.String("OPTIONS"),
				Authorization: pulumi.String("NONE"),
				RestApi:       gateway.ID(),
				ResourceId:    apiresource.ID(),
			},
			pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, authorizer, function}),
		)
		if err != nil {
			return err
		}
		corsIntegration, err := apigateway.NewIntegration(ctx, "CORSIntegration",
			&apigateway.IntegrationArgs{
				HttpMethod:            pulumi.String("OPTIONS"),
				IntegrationHttpMethod: pulumi.String("POST"),
				ResourceId:            apiresource.ID(),
				RestApi:               gateway.ID(),
				Type:                  pulumi.String("MOCK"),
				RequestTemplates: pulumi.StringMap{
					"application/json": pulumi.String("{ \"statusCode\": 200 }"),
				},
			},
			pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, authorizer, function, authFunction, method, corsmethod}),
		)
		if err != nil {
			return err
		}
		corsMethodResp, err := apigateway.NewMethodResponse(ctx, "response_method",
			&apigateway.MethodResponseArgs{
				HttpMethod: pulumi.Sprintf("%s", corsmethod.HttpMethod),
				// ResponseModels: pulumi.StringMap{
				// 	"application/json": pulumi.String("Empty"),
				// },
				ResponseParameters: pulumi.BoolMap{
					"method.response.header.Access-Control-Allow-Headers": pulumi.Bool(true),
					"method.response.header.Access-Control-Allow-Methods": pulumi.Bool(true),
					"method.response.header.Access-Control-Allow-Origin":  pulumi.Bool(true),
				},
				ResourceId: apiresource.ID(),
				RestApi:    gateway.ID(),
				StatusCode: pulumi.String("200"),
			},
			pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, authorizer, function, authFunction, method, corsIntegration, corsmethod}),
		)
		if err != nil {
			return err
		}
		corsIntegResp, err := apigateway.NewIntegrationResponse(ctx, "response_integration",
			&apigateway.IntegrationResponseArgs{
				HttpMethod: pulumi.Sprintf("%s", corsmethod.HttpMethod),
				RestApi:    gateway.ID(),
				ResourceId: pulumi.Sprintf("%s", corsmethod.ResourceId),
				ResponseParameters: pulumi.StringMap{
					"method.response.header.Access-Control-Allow-Headers": pulumi.String("'Origin,Accept,Authorization,Content-Type,User-Agent,X-Api-Key,Referer,Accept-Encoding,Accept-Language,Sec-Fetch-Dest,Sec-Fetch-Mode,Sec-Fetch-Site'"),
					"method.response.header.Access-Control-Allow-Methods": pulumi.String("'OPTIONS,DELETE,GET,HEAD,PATCH,POST,PUT'"),
					"method.response.header.Access-Control-Allow-Origin":  pulumi.String("'*'"),
				},
				StatusCode: pulumi.Sprintf("%s", corsMethodResp.StatusCode),
			},
			pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, authorizer, function, authFunction, method, corsMethodResp, corsIntegration, corsmethod}),
		)
		if err != nil {
			return err
		}
		////////////////////////////////////////////////////////////////////////////////////////////////////////////////
		// Add a resource based policy to the Lambda function.
		// This is the final step and allows AWS API Gateway to communicate with the AWS Lambda function
		authpermission, err := lambda.NewPermission(ctx, "AuthAPIPermission",
			&lambda.PermissionArgs{
				Action:    pulumi.String("lambda:InvokeFunction"),
				Function:  authFunction.Name,
				Principal: pulumi.String("apigateway.amazonaws.com"),
				SourceArn: pulumi.Sprintf("arn:aws:execute-api:%s:%s:%s/*/*", region.Name, account.AccountId, gateway.ID()),
			},
			pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, authorizer, authFunction, function, authFunction, integration}),
		)
		if err != nil {
			return err
		}
		permission, err := lambda.NewPermission(ctx, "APIPermission",
			&lambda.PermissionArgs{
				Action:    pulumi.String("lambda:InvokeFunction"),
				Function:  function.Name,
				Principal: pulumi.String("apigateway.amazonaws.com"),
				SourceArn: pulumi.Sprintf("arn:aws:execute-api:%s:%s:%s/*/*/*", region.Name, account.AccountId, gateway.ID()),
			},
			pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, authorizer, function, authFunction, integration}),
		)
		if err != nil {
			return err
		}

		// Create a new deployment
		_, err = apigateway.NewDeployment(ctx, "APIDeployment",
			&apigateway.DeploymentArgs{
				Description:      pulumi.String("UpperCase API deployment"),
				RestApi:          gateway.ID(),
				StageDescription: pulumi.String("Production"),
				StageName:        pulumi.String("prod"),
			},
			pulumi.DependsOn([]pulumi.Resource{gateway, apiresource, authorizer, function, integration, permission, authpermission, method, corsMethodResp, corsIntegration, corsmethod, corsIntegResp}),
		)
		if err != nil {
			return err
		}

		ctx.Export("invocation URL", pulumi.Sprintf("https://%s.execute-api.%s.amazonaws.com/prod/{message}", gateway.ID(), region.Name))

		return nil
	})
}
