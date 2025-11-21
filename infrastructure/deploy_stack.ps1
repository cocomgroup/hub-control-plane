###############################################################################
# CloudFormation Stack Deployment Script (PowerShell) - Simplified
# Deploys Go application with DynamoDB and Redis
###############################################################################

param(
    [string]$StackName = "hub-control-plane",
    [string]$TemplateFile = "./infrastructure.yaml",
    [string]$Region = "us-east-1",
    [string]$Environment = "dev",
    [string]$InstanceType = "t3.micro",
    [string]$KeyName = "",
    [string]$SSHLocation = "",
    [string]$DomainName = "",
    [switch]$Status,
    [switch]$Outputs,
    [switch]$Help
)

# Function to print colored output
function Write-Status {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Green
}

function Write-Error-Custom {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

function Write-Warning-Custom {
    param([string]$Message)
    Write-Host "[WARNING] $Message" -ForegroundColor Yellow
}

# Function to check if AWS CLI is installed
function Test-AwsCli {
    try {
        $version = aws --version 2>&1
        Write-Status "AWS CLI found: $version"
        return $true
    }
    catch {
        Write-Error-Custom "AWS CLI is not installed. Please install it first."
        Write-Host "Download from: https://aws.amazon.com/cli/" -ForegroundColor Yellow
        return $false
    }
}

# Function to validate template
function Test-CloudFormationTemplate {
    param([string]$TemplatePath, [string]$AwsRegion)
    
    Write-Status "Validating CloudFormation template..."
    try {
        $result = aws cloudformation validate-template `
            --template-body "file://$TemplatePath" `
            --region $AwsRegion 2>&1
        
        if ($LASTEXITCODE -eq 0) {
            Write-Status "Template validation successful"
            return $true
        }
        else {
            Write-Error-Custom "Template validation failed: $result"
            return $false
        }
    }
    catch {
        Write-Error-Custom "Template validation failed: $_"
        return $false
    }
}

# Function to check if stack exists
function Test-StackExists {
    param([string]$Stack, [string]$AwsRegion)
    
    try {
        $result = aws cloudformation describe-stacks `
            --stack-name $Stack `
            --region $AwsRegion 2>&1
        return $LASTEXITCODE -eq 0
    }
    catch {
        return $false
    }
}

# Function to get available EC2 key pairs
function Get-KeyPairs {
    param([string]$AwsRegion)
    
    Write-Status "Fetching available EC2 key pairs..."
    try {
        $keyPairs = aws ec2 describe-key-pairs `
            --region $AwsRegion `
            --query 'KeyPairs[*].KeyName' `
            --output text
        
        if ($LASTEXITCODE -eq 0) {
            return $keyPairs
        }
        else {
            Write-Warning-Custom "No key pairs found or unable to retrieve them"
            return $null
        }
    }
    catch {
        Write-Error-Custom "Failed to retrieve key pairs: $_"
        return $null
    }
}

# Function to deploy stack
function Deploy-Stack {
    param(
        [string]$Stack,
        [string]$Template,
        [string]$AwsRegion,
        [string]$Env,
        [string]$Instance,
        [string]$Key,
        [string]$SSH,
        [string]$Domain
    )

    Write-Host ""
    Write-Host "=====================================" -ForegroundColor Cyan
    Write-Status "Starting deployment of stack: $Stack"
    Write-Status "Environment: $Env"
    Write-Status "Region: $AwsRegion"
    Write-Host "=====================================" -ForegroundColor Cyan
    Write-Host ""

    # Check if stack exists
    $stackExists = Test-StackExists -Stack $Stack -AwsRegion $AwsRegion
    
    if ($stackExists) {
        Write-Warning-Custom "Stack '$Stack' already exists. Updating stack..."
        $action = "update-stack"
    }
    else {
        Write-Status "Creating new stack '$Stack'..."
        $action = "create-stack"
    }

    # Build parameters
    $parameters = @(
        "ParameterKey=EnvironmentName,ParameterValue=$Env",
        "ParameterKey=InstanceType,ParameterValue=$Instance",
        "ParameterKey=KeyName,ParameterValue=$Key",
        "ParameterKey=SSHLocation,ParameterValue=$SSH"
    )

    if (-not [string]::IsNullOrWhiteSpace($Domain)) {
        $parameters += "ParameterKey=DomainName,ParameterValue=$Domain"
        Write-Status "Custom domain will be configured: $Domain"
    }

    try {
        if ($action -eq "create-stack") {
            Write-Status "Initiating stack creation..."
            $createResult = aws cloudformation create-stack `
                --stack-name $Stack `
                --template-body "file://$Template" `
                --parameters $parameters `
                --capabilities CAPABILITY_NAMED_IAM `
                --region $AwsRegion `
                --tags "Key=Environment,Value=$Env" "Key=ManagedBy,Value=CloudFormation" 2>&1

            if ($LASTEXITCODE -ne 0) {
                Write-Error-Custom "Failed to create stack: $createResult"
                return $false
            }

            Write-Status "Stack creation initiated successfully"
            Write-Status "Waiting for stack creation to complete (this may take 5-10 minutes)..."
            Write-Host "You can monitor progress in the AWS Console: CloudFormation > Stacks > $Stack" -ForegroundColor Yellow
            Write-Host ""
            
            $waitResult = aws cloudformation wait stack-create-complete `
                --stack-name $Stack `
                --region $AwsRegion 2>&1

            if ($LASTEXITCODE -ne 0) {
                Write-Error-Custom "Stack creation failed or timed out"
                Write-Status "Checking stack events for errors..."
                
                aws cloudformation describe-stack-events `
                    --stack-name $Stack `
                    --region $AwsRegion `
                    --query 'StackEvents[?ResourceStatus==`CREATE_FAILED`].[LogicalResourceId,ResourceStatusReason]' `
                    --output table
                
                return $false
            }
        }
        else {
            Write-Status "Initiating stack update..."
            $updateResult = aws cloudformation update-stack `
                --stack-name $Stack `
                --template-body "file://$Template" `
                --parameters $parameters `
                --capabilities CAPABILITY_IAM `
                --region $AwsRegion 2>&1

            if ($LASTEXITCODE -ne 0) {
                if ($updateResult -match "No updates are to be performed") {
                    Write-Warning-Custom "No changes detected. Stack is already up to date."
                    return $true
                }
                else {
                    Write-Error-Custom "Failed to update stack: $updateResult"
                    return $false
                }
            }

            Write-Status "Stack update initiated successfully"
            Write-Status "Waiting for stack update to complete (this may take 5-10 minutes)..."
            Write-Host ""
            
            $waitResult = aws cloudformation wait stack-update-complete `
                --stack-name $Stack `
                --region $AwsRegion 2>&1

            if ($LASTEXITCODE -ne 0) {
                Write-Error-Custom "Stack update failed or timed out"
                Write-Status "Checking stack events for errors..."
                
                aws cloudformation describe-stack-events `
                    --stack-name $Stack `
                    --region $AwsRegion `
                    --query 'StackEvents[?ResourceStatus==`UPDATE_FAILED`].[LogicalResourceId,ResourceStatusReason]' `
                    --output table
                
                return $false
            }
        }

        Write-Host ""
        Write-Host "=====================================" -ForegroundColor Green
        Write-Status "Stack deployment completed successfully!"
        Write-Host "=====================================" -ForegroundColor Green
        Write-Host ""
        
        return $true
    }
    catch {
        Write-Error-Custom "Stack deployment failed with exception: $_"
        return $false
    }
}

# Function to display stack outputs
function Show-StackOutputs {
    param([string]$Stack, [string]$AwsRegion)
    
    Write-Host ""
    Write-Host "=====================================" -ForegroundColor Cyan
    Write-Status "Stack Outputs for: $Stack"
    Write-Host "=====================================" -ForegroundColor Cyan
    Write-Host ""
    
    try {
        $outputs = aws cloudformation describe-stacks `
            --stack-name $Stack `
            --region $AwsRegion `
            --query 'Stacks[0].Outputs[*].[OutputKey,OutputValue,Description]' `
            --output table 2>&1
        
        if ($LASTEXITCODE -eq 0) {
            Write-Host $outputs
            Write-Host ""
            
            # Extract and highlight important URLs
            $jsonOutputs = aws cloudformation describe-stacks `
                --stack-name $Stack `
                --region $AwsRegion `
                --query 'Stacks[0].Outputs' `
                --output json | ConvertFrom-Json
            
            Write-Host "Quick Access:" -ForegroundColor Yellow
            foreach ($output in $jsonOutputs) {
                if ($output.OutputKey -eq "CloudFrontURL") {
                    Write-Host "  Frontend URL: " -NoNewline -ForegroundColor Green
                    Write-Host $output.OutputValue -ForegroundColor Cyan
                }
                elseif ($output.OutputKey -eq "ALBEndpoint") {
                    Write-Host "  Backend API:  " -NoNewline -ForegroundColor Green
                    Write-Host "http://$($output.OutputValue)" -ForegroundColor Cyan
                }
                elseif ($output.OutputKey -eq "BackendInstancePublicIP") {
                    Write-Host "  SSH Access:   " -NoNewline -ForegroundColor Green
                    Write-Host "ssh -i your-key.pem ubuntu@$($output.OutputValue)" -ForegroundColor Cyan
                }
            }
            Write-Host ""
        }
        else {
            Write-Error-Custom "Failed to retrieve stack outputs: $outputs"
        }
    }
    catch {
        Write-Error-Custom "Failed to retrieve stack outputs: $_"
    }
}

# Function to get stack status
function Get-StackStatus {
    param([string]$Stack, [string]$AwsRegion)
    
    Write-Host ""
    Write-Host "=====================================" -ForegroundColor Cyan
    Write-Status "Checking status for stack: $Stack"
    Write-Host "=====================================" -ForegroundColor Cyan
    Write-Host ""
    
    try {
        $status = aws cloudformation describe-stacks `
            --stack-name $Stack `
            --region $AwsRegion `
            --query 'Stacks[0].StackStatus' `
            --output text 2>&1
        
        if ($LASTEXITCODE -eq 0) {
            $color = switch -Regex ($status) {
                ".*COMPLETE$" { "Green" }
                ".*IN_PROGRESS$" { "Yellow" }
                ".*FAILED$" { "Red" }
                default { "White" }
            }
            
            Write-Host "Stack Status: " -NoNewline
            Write-Host $status -ForegroundColor $color
            
            # Get additional info
            $stackInfo = aws cloudformation describe-stacks `
                --stack-name $Stack `
                --region $AwsRegion `
                --query 'Stacks[0].[CreationTime,LastUpdatedTime]' `
                --output text
            
            if ($stackInfo) {
                $times = $stackInfo -split "`t"
                Write-Host "Created: $($times[0])" -ForegroundColor Gray
                if ($times[1] -ne "None") {
                    Write-Host "Last Updated: $($times[1])" -ForegroundColor Gray
                }
            }
        }
        else {
            Write-Host "Stack Status: " -NoNewline
            Write-Host "NOT_FOUND" -ForegroundColor Yellow
            Write-Host "Stack '$Stack' does not exist in region $AwsRegion" -ForegroundColor Gray
        }
    }
    catch {
        Write-Host "Stack Status: " -NoNewline
        Write-Host "ERROR" -ForegroundColor Red
        Write-Error-Custom "Failed to get stack status: $_"
    }
    Write-Host ""
}

# Interactive deployment
function Start-InteractiveDeployment {
    param([string]$Stack, [string]$Template, [string]$AwsRegion)

    Write-Host ""
    Write-Host "=====================================" -ForegroundColor Cyan
    Write-Host " CloudFormation Stack Deployment" -ForegroundColor Cyan
    Write-Host " Simplified: EC2 + S3 + ALB + CloudFront" -ForegroundColor Cyan
    Write-Host "=====================================" -ForegroundColor Cyan
    Write-Host ""

    # Get environment name
    Write-Host "Environment Configuration:" -ForegroundColor Yellow
    $env = Read-Host "Environment name (dev/staging/prod) [prod]"
    if ([string]::IsNullOrWhiteSpace($env)) { $env = "prod" }

    # Get instance type
    Write-Host ""
    Write-Host "Instance Configuration:" -ForegroundColor Yellow
    Write-Host "  1) t3.micro   - 2 vCPU, 1 GB RAM  (~`$7/month)"
    Write-Host "  2) t3.small   - 2 vCPU, 2 GB RAM  (~`$15/month)"
    Write-Host "  3) t3.medium  - 2 vCPU, 4 GB RAM  (~`$30/month) [Default]"
    Write-Host "  4) t3.large   - 2 vCPU, 8 GB RAM  (~`$60/month)"
    $instanceChoice = Read-Host "Select instance type [3]"
    if ([string]::IsNullOrWhiteSpace($instanceChoice)) { $instanceChoice = "3" }
    
    $instance = switch ($instanceChoice) {
        "1" { "t3.micro" }
        "2" { "t3.small" }
        "3" { "t3.medium" }
        "4" { "t3.large" }
        default { "t3.medium" }
    }

    # Get EC2 key pair
    Write-Host ""
    Write-Host "SSH Key Configuration:" -ForegroundColor Yellow
    $keyPairs = Get-KeyPairs -AwsRegion $AwsRegion
    if ($keyPairs) {
        Write-Host "Available key pairs: $keyPairs" -ForegroundColor Gray
    }
    else {
        Write-Warning-Custom "No key pairs found. You may need to create one first."
        Write-Host "Create a key pair: aws ec2 create-key-pair --key-name my-key --query 'KeyMaterial' --output text > my-key.pem" -ForegroundColor Gray
    }
    $key = Read-Host "Enter EC2 Key Pair name"
    
    if ([string]::IsNullOrWhiteSpace($key)) {
        Write-Error-Custom "Key pair name is required"
        exit 1
    }

    # Get SSH location
    Write-Host ""
    Write-Host "Network Security:" -ForegroundColor Yellow
    Write-Host "SSH access will be restricted to the CIDR range you specify" -ForegroundColor Gray
    Write-Host "Examples: Your IP (203.0.113.25/32), Office network (10.0.0.0/8), Anywhere (0.0.0.0/0)" -ForegroundColor Gray
    $ssh = Read-Host "SSH access CIDR [0.0.0.0/0]"
    if ([string]::IsNullOrWhiteSpace($ssh)) { $ssh = "0.0.0.0/0" }

    # Get optional domain name
    Write-Host ""
    Write-Host "Domain Configuration (Optional):" -ForegroundColor Yellow
    Write-Host "Leave blank to use CloudFront's default domain" -ForegroundColor Gray
    $domain = Read-Host "Custom domain name (optional, press enter to skip)"

    # Confirm deployment
    Write-Host ""
    Write-Host "=====================================" -ForegroundColor Cyan
    Write-Host " Deployment Summary" -ForegroundColor Cyan
    Write-Host "=====================================" -ForegroundColor Cyan
    Write-Host "Stack Name:       " -NoNewline; Write-Host $Stack -ForegroundColor White
    Write-Host "Template:         " -NoNewline; Write-Host $Template -ForegroundColor White
    Write-Host "Environment:      " -NoNewline; Write-Host $env -ForegroundColor White
    Write-Host "Region:           " -NoNewline; Write-Host $AwsRegion -ForegroundColor White
    Write-Host "Instance Type:    " -NoNewline; Write-Host $instance -ForegroundColor White
    Write-Host "Key Pair:         " -NoNewline; Write-Host $key -ForegroundColor White
    Write-Host "SSH Access:       " -NoNewline; Write-Host $ssh -ForegroundColor White
    if (-not [string]::IsNullOrWhiteSpace($domain)) {
        Write-Host "Domain Name:      " -NoNewline; Write-Host $domain -ForegroundColor White
    }
    Write-Host "=====================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Resources to be created:" -ForegroundColor Yellow
    Write-Host "  - VPC with public subnets" -ForegroundColor Gray
    Write-Host "  - Application Load Balancer" -ForegroundColor Gray
    Write-Host "  - EC2 instance ($instance)" -ForegroundColor Gray
    Write-Host "  - 2 S3 buckets (static assets + data)" -ForegroundColor Gray
    Write-Host "  - CloudFront distribution" -ForegroundColor Gray
    Write-Host "  - Security groups and IAM roles" -ForegroundColor Gray
    Write-Host ""
    
    $confirm = Read-Host "Proceed with deployment? (yes/no) [yes]"
    if ([string]::IsNullOrWhiteSpace($confirm)) { $confirm = "yes" }

    if ($confirm -ne "yes" -and $confirm -ne "y") {
        Write-Warning-Custom "Deployment cancelled by user"
        exit 0
    }

    # Deploy the stack
    $success = Deploy-Stack `
        -Stack $Stack `
        -Template $Template `
        -AwsRegion $AwsRegion `
        -Env $env `
        -Instance $instance `
        -Key $key `
        -SSH $ssh `
        -Domain $domain

    if ($success) {
        Show-StackOutputs -Stack $Stack -AwsRegion $AwsRegion
        
        Write-Host ""
        Write-Host "Next Steps:" -ForegroundColor Yellow
        Write-Host "  1. Deploy your Go backend to the EC2 instance" -ForegroundColor Gray
        Write-Host "  2. Build and upload your Svelte frontend to S3" -ForegroundColor Gray
        Write-Host "  3. Access your application via the CloudFront URL above" -ForegroundColor Gray
        Write-Host ""
    }
}

# Show help
function Show-Help {
    Write-Host @"

====================================
CloudFormation Stack Deployment
Simplified: EC2 + S3 + ALB + CloudFront
====================================

Usage: .\deploy-stack-simple.ps1 [OPTIONS]

Options:
  -StackName NAME         CloudFormation stack name (default: webapp-simple)
  -TemplateFile FILE      CloudFormation template file (default: webapp-simple.yaml)
  -Region REGION          AWS region (default: us-east-1)
  -Environment ENV        Environment name (dev/staging/prod)
  -InstanceType TYPE      EC2 instance type (t3.micro/small/medium/large)
  -KeyName NAME           EC2 key pair name (required for deployment)
  -SSHLocation CIDR       SSH access CIDR (default: 0.0.0.0/0)
  -DomainName NAME        Custom domain name (optional)
  -Status                 Get current stack status
  -Outputs                Display stack outputs
  -Help                   Show this help message

Examples:

  # Interactive mode (recommended)
  .\deploy-stack-simple.ps1

  # Quick deployment with defaults
  .\deploy-stack-simple.ps1 -KeyName my-ec2-key

  # Full configuration
  .\deploy-stack-simple.ps1 ``
    -StackName my-app ``
    -Environment prod ``
    -InstanceType t3.medium ``
    -KeyName my-ec2-key ``
    -SSHLocation "203.0.113.0/24"

  # With custom domain
  .\deploy-stack-simple.ps1 ``
    -KeyName my-ec2-key ``
    -DomainName example.com

  # Check stack status
  .\deploy-stack-simple.ps1 -Status

  # Display stack outputs
  .\deploy-stack-simple.ps1 -Outputs

Resources Created:
  - VPC with 2 public subnets
  - Application Load Balancer
  - EC2 instance for Go backend
  - S3 bucket for static assets (public)
  - S3 bucket for application data (private)
  - CloudFront distribution
  - Security groups and IAM roles

NOT Included (compared to full template):
  - DynamoDB table
  - ElastiCache Redis
  - Private subnets

"@
}

# Main execution
function Main {
    # Show help if requested
    if ($Help) {
        Show-Help
        exit 0
    }

    # Check AWS CLI
    if (-not (Test-AwsCli)) {
        exit 1
    }

    # Check if template file exists
    if (-not (Test-Path $TemplateFile)) {
        Write-Error-Custom "Template file not found: $TemplateFile"
        Write-Host "Please ensure the template file is in the current directory" -ForegroundColor Yellow
        exit 1
    }

    # Handle status check
    if ($Status) {
        Get-StackStatus -Stack $StackName -AwsRegion $Region
        exit 0
    }

    # Handle outputs display
    if ($Outputs) {
        Show-StackOutputs -Stack $StackName -AwsRegion $Region
        exit 0
    }

    # Validate template
    if (-not (Test-CloudFormationTemplate -TemplatePath $TemplateFile -AwsRegion $Region)) {
        exit 1
    }

    # Check if running in interactive mode
    if ([string]::IsNullOrWhiteSpace($KeyName)) {
        Start-InteractiveDeployment -Stack $StackName -Template $TemplateFile -AwsRegion $Region
    }
    else {
        # Command line mode
        if ([string]::IsNullOrWhiteSpace($Environment)) { $Environment = "prod" }
        if ([string]::IsNullOrWhiteSpace($InstanceType)) { $InstanceType = "t3.medium" }
        if ([string]::IsNullOrWhiteSpace($SSHLocation)) { $SSHLocation = "0.0.0.0/0" }

        Write-Host ""
        Write-Host "Running in command-line mode..." -ForegroundColor Cyan
        
        $success = Deploy-Stack `
            -Stack $StackName `
            -Template $TemplateFile `
            -AwsRegion $Region `
            -Env $Environment `
            -Instance $InstanceType `
            -Key $KeyName `
            -SSH $SSHLocation `
            -Domain $DomainName

        if ($success) {
            Show-StackOutputs -Stack $StackName -AwsRegion $Region
        }
        else {
            Write-Host ""
            Write-Error-Custom "Deployment failed. Check the error messages above."
            exit 1
        }
    }
}

# Run main function
Main