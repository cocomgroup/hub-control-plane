# AWS CloudFormation deployment script for Hub Control Plane
# Single-table DynamoDB with Redis cache infrastructure

# Configuration
$STACK_NAME = "hub-control-plane-stack"
$TEMPLATE_FILE = "infrastructure.yaml"
$REGION = "us-east-1"
$ENVIRONMENT = "dev"

function Write-Info { 
    Write-Host $args -ForegroundColor Cyan 
}

function Write-Success { 
    Write-Host $args -ForegroundColor Green 
}

function Write-ErrorMsg { 
    Write-Host $args -ForegroundColor Red 
}

Write-Info "================================================"
Write-Info "Hub Control Plane - Infrastructure Deployment"
Write-Info "================================================"
Write-Host ""

# Check if AWS CLI is installed
try {
    $null = Get-Command aws -ErrorAction Stop
} catch {
    Write-ErrorMsg "Error: AWS CLI is not installed"
    exit 1
}

# Check if template exists
if (-not (Test-Path $TEMPLATE_FILE)) {
    Write-ErrorMsg "Error: Template file $TEMPLATE_FILE not found"
    exit 1
}

# Validate the template
Write-Info "Validating CloudFormation template..."
$ErrorActionPreference = "SilentlyContinue"
$validateResult = aws cloudformation validate-template --template-body "file://$TEMPLATE_FILE" --region $REGION 2>&1
$ErrorActionPreference = "Continue"

if ($LASTEXITCODE -eq 0) {
    Write-Success "Template validation successful"
} else {
    Write-ErrorMsg "Template validation failed"
    Write-ErrorMsg $validateResult
    exit 1
}

# Check if stack exists - suppress all error output
Write-Info "Checking if stack exists..."
$stackExists = $false

$ErrorActionPreference = "SilentlyContinue"
$stackCheck = aws cloudformation describe-stacks --stack-name $STACK_NAME --region $REGION 2>&1 | Out-Null
$checkExitCode = $LASTEXITCODE
$ErrorActionPreference = "Continue"

if ($checkExitCode -eq 0) {
    $stackExists = $true
    Write-Info "Stack exists, will update"
} else {
    $stackExists = $false
    Write-Info "Stack does not exist, will create new stack"
}

if (-not $stackExists) {
    Write-Info "Creating new stack: $STACK_NAME"
    Write-Host ""
    
    $createResult = aws cloudformation create-stack --stack-name $STACK_NAME --template-body "file://$TEMPLATE_FILE" --parameters "ParameterKey=Environment,ParameterValue=$ENVIRONMENT" --capabilities CAPABILITY_IAM CAPABILITY_NAMED_IAM --region $REGION --tags "Key=Environment,Value=$ENVIRONMENT" "Key=Project,Value=HubControlPlane" "Key=ManagedBy,Value=CloudFormation" 2>&1
    
    if ($LASTEXITCODE -ne 0) {
        Write-ErrorMsg "Stack creation failed"
        Write-ErrorMsg $createResult
        exit 1
    }
    
    Write-Success "Stack creation initiated"
    Write-Info "Stack ID: $createResult"
    Write-Info ""
    Write-Info "Waiting for stack creation to complete (this may take 5-10 minutes)..."
    Write-Info "Creating VPC, subnets, DynamoDB table, and Redis cluster..."
    Write-Host ""
    
    aws cloudformation wait stack-create-complete --stack-name $STACK_NAME --region $REGION
    
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Stack created successfully!"
    } else {
        Write-ErrorMsg "Stack creation did not complete successfully"
        Write-Info ""
        Write-Info "Recent stack events:"
        aws cloudformation describe-stack-events --stack-name $STACK_NAME --region $REGION --max-items 10 --query 'StackEvents[*].[Timestamp,ResourceStatus,ResourceType,LogicalResourceId,ResourceStatusReason]' --output table
        exit 1
    }
}

if ($stackExists) {
    Write-Info "Updating existing stack: $STACK_NAME"
    Write-Host ""
    
    $updateResult = aws cloudformation update-stack --stack-name $STACK_NAME --template-body "file://$TEMPLATE_FILE" --parameters "ParameterKey=Environment,ParameterValue=$ENVIRONMENT" --capabilities CAPABILITY_IAM CAPABILITY_NAMED_IAM --region $REGION 2>&1 | Out-String
    
    if ($updateResult -match "No updates are to be performed") {
        Write-Info "No updates needed - stack is up to date"
    } elseif ($LASTEXITCODE -eq 0) {
        Write-Success "Stack update initiated"
        Write-Info "Waiting for stack update to complete..."
        Write-Host ""
        
        aws cloudformation wait stack-update-complete --stack-name $STACK_NAME --region $REGION
        
        if ($LASTEXITCODE -eq 0) {
            Write-Success "Stack updated successfully!"
        } else {
            Write-ErrorMsg "Stack update did not complete successfully"
            Write-Info ""
            Write-Info "Recent stack events:"
            aws cloudformation describe-stack-events --stack-name $STACK_NAME --region $REGION --max-items 10 --query 'StackEvents[*].[Timestamp,ResourceStatus,ResourceType,LogicalResourceId,ResourceStatusReason]' --output table
            exit 1
        }
    } else {
        Write-ErrorMsg "Stack update failed"
        Write-ErrorMsg $updateResult
        exit 1
    }
}

Write-Host ""
Write-Host ""
Write-Info "==============================================="
Write-Info "Stack Outputs"
Write-Info "==============================================="
Write-Host ""

aws cloudformation describe-stacks --stack-name $STACK_NAME --region $REGION --query 'Stacks[0].Outputs[*].[OutputKey,OutputValue]' --output table

Write-Host ""
Write-Success "================================================"
Write-Success "Deployment Complete!"
Write-Success "================================================"
Write-Host ""

Write-Info "Connection Information:"
Write-Host ""

$outputsJson = aws cloudformation describe-stacks --stack-name $STACK_NAME --region $REGION --query 'Stacks[0].Outputs' 2>$null

if ($outputsJson) {
    $outputs = $outputsJson | ConvertFrom-Json
    
    foreach ($output in $outputs) {
        if ($output.OutputKey -eq "RedisEndpoint") {
            Write-Host "  Redis Endpoint: " -NoNewline
            Write-Host $output.OutputValue -ForegroundColor Yellow
        }
        if ($output.OutputKey -eq "RedisPort") {
            Write-Host "  Redis Port: " -NoNewline
            Write-Host $output.OutputValue -ForegroundColor Yellow
        }
        if ($output.OutputKey -eq "DynamoDBTableName") {
            Write-Host "  DynamoDB Table: " -NoNewline
            Write-Host $output.OutputValue -ForegroundColor Yellow
        }
    }
}

Write-Host ""
Write-Info "To delete this stack, run:"
Write-Host "  aws cloudformation delete-stack --stack-name $STACK_NAME --region $REGION" -ForegroundColor Gray
Write-Host ""