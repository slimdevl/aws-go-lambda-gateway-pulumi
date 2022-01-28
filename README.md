# AWS Golang Lambda With API Gateway and Authorizer

This example creates a lambda that does a simple `ToUpper` on the path input of an API request and returns it.

## Deploying the App

To deploy your infrastructure, follow the below steps.

### Prerequisites

1. [Install Pulumi](https://www.pulumi.com/docs/get-started/install/)
2. [Configure AWS Credentials](https://www.pulumi.com/docs/intro/cloud-providers/aws/setup/)
3. [Clone aws-go-lambda](https://github.com/aws/aws-lambda-go)

### Steps

After cloning this repo, run these commands from the working directory:

1. Build the handler:

	- For developers on Linux and macOS:

		```bash
		make build
		```

	- For developers on Windows:

		- Get the `build-lambda-zip` tool:

			```bash
			set GO111MODULE=on
			go.exe get -u github.com/aws/aws-lambda-go/cmd/build-lambda-zip
			```

		- Use the tool from your GOPATH:

			```bash
			set GOOS=linux
			set GOARCH=amd64
			set CGO_ENABLED=0
			go build -o handler\handler handler\handler.go
			%USERPROFILE%\Go\bin\build-lambda-zip.exe -o handler\handler.zip handler\handler
			```


2. Create a new Pulumi stack, which is an isolated deployment target for this example:

	```bash
	pulumi stack init
	```

3. Set the required configuration variables for this program:
	```bash
	$ pulumi config set aws:region us-east-1
	```

4. Execute the Pulumi program to create our lambda:

	```bash
	$ pulumi up
	Previewing update (dev):
         Type                           Name                          Plan
     +   pulumi:pulumi:Stack            go-lambda-gateway-dev-lambda  create
     +   ├─ aws:iam:Role                task-exec-role                create
     +   ├─ aws:apigateway:RestApi      UpperCaseGateway              create
     +   ├─ aws:iam:RolePolicy          lambda-log-policy             create
     +   ├─ aws:apigateway:Resource     UpperAPI                      create
     +   ├─ aws:lambda:Function         basicLambda                   create
     +   ├─ aws:lambda:Function         authFunction                  create
     +   ├─ aws:apigateway:Authorizer   authorizer                    create
     +   ├─ aws:apigateway:Method       AnyMethod                     create
     +   ├─ aws:apigateway:Integration  LambdaIntegration             create
     +   ├─ aws:lambda:Permission       APIPermission                 create
     +   ├─ aws:lambda:Permission       AuthAPIPermission             create
     +   └─ aws:apigateway:Deployment   APIDeployment                 create

    Resources:
        + 13 to create

	Do you want to perform this update? yes
	Updating (dev):
         Type                           Name                          Status
     +   pulumi:pulumi:Stack            go-lambda-gateway-dev-lambda  created
     +   ├─ aws:iam:Role                task-exec-role                created
     +   ├─ aws:apigateway:RestApi      UpperCaseGateway              created
     +   ├─ aws:apigateway:Resource     UpperAPI                      created
     +   ├─ aws:iam:RolePolicy          lambda-log-policy             created
     +   ├─ aws:lambda:Function         authFunction                  created
     +   ├─ aws:lambda:Function         basicLambda                   created
     +   ├─ aws:apigateway:Authorizer   authorizer                    created
     +   ├─ aws:apigateway:Method       AnyMethod                     created
     +   ├─ aws:apigateway:Integration  LambdaIntegration             created
     +   ├─ aws:lambda:Permission       APIPermission                 created
     +   ├─ aws:lambda:Permission       AuthAPIPermission             created
     +   └─ aws:apigateway:Deployment   APIDeployment                 created

    Outputs:
        invocation URL: "https://e3tv36udd3.execute-api.us-east-1.amazonaws.com/prod/{message}"

    Resources:
        + 13 created

    Duration: 28s
	```

5. Call our lambda function from the cli:
    ```bash
    $ curl https://<gateway-id>.execute-api.us-east-1.amazonaws.com/prod/helloworld
    {"message":"Unauthorized"}
    ```

    ```bash
	curl https://<gateway-id>.execute-api.us-east-1.amazonaws.com/prod/helloworld -H "authorizationToken:deny"
    {"Message":"User is not authorized to access this resource with an explicit deny"}
	```

	```bash
	curl https://<gateway-id>.execute-api.us-east-1.amazonaws.com/prod/helloworld -H "authorizationToken: allow"
	HELLOWORLD%
	```

    ```bash
    SECRET=$(pulumi stack output --show-secrets key)
    curl https://<gateway-id>.execute-api.us-east-1.amazonaws.com/prod/message -H "authorizationToken: ${SECRET}"
	HELLOWORLD%
    ```

6. From there, feel free to experiment. Simply making edits, rebuilding your handler, and running `pulumi up` will update your lambda.

7. Afterwards, destroy your stack and remove it:

	```bash
	pulumi destroy --yes
	pulumi stack rm --yes
	```
