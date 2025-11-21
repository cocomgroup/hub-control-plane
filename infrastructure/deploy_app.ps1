# deploy-app.ps1 - PowerShell version for Windows
# Deploy Go backend to AWS

param(
    [Parameter(Mandatory=$false)]
    [string]$StackName = "hub-control-plane-stack",
    
    [Parameter(Mandatory=$false)]
    [string]$BackendDir = "./backend",
    
    [Parameter(Mandatory=$false)]
    [string]$KeyFile = "~/.ssh/my-key.pem"
)

Write-Host "=== Full Stack Deployment (Svelte + Go) ===" -ForegroundColor Green
Write-Host ""


if (-not (Test-Path $BackendDir)) {
    Write-Host "Error: Backend directory not found: $BackendDir" -ForegroundColor Red
    exit 1
}

Write-Host "Getting stack outputs..." -ForegroundColor Blue

# Get stack outputs
$outputs = aws cloudformation describe-stacks --stack-name $StackName --query 'Stacks[0].Outputs' --output json | ConvertFrom-Json

if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Could not get stack outputs. Is the stack deployed?" -ForegroundColor Red
    exit 1
}

# Extract values
$backendIp = ($outputs | Where-Object {$_.OutputKey -eq "BackendInstancePublicIP"}).OutputValue
$albEndpoint = ($outputs | Where-Object {$_.OutputKey -eq "ALBEndpoint"}).OutputValue

Write-Host "Stack outputs retrieved" -ForegroundColor Green
Write-Host "  Static Bucket: $staticBucket"
Write-Host "  Backend IP: $backendIp"
Write-Host "  ALB Endpoint: $albEndpoint"
Write-Host ""

# ==========================================
# Deploy Backend (Go)
# ==========================================
Write-Host "Step 4: Building Go backend..." -ForegroundColor Green

Push-Location $BackendDir

# Check if main.go exists
if (-not (Test-Path "main.go") -and -not (Test-Path "cmd/main.go") -and -not (Test-Path "cmd/server/main.go")) {
    Write-Host "Error: main.go not found in backend directory" -ForegroundColor Red
    Pop-Location
    exit 1
}

Write-Host "Compiling Go binary for Linux..." -ForegroundColor Yellow

# Build for Linux
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o backend -ldflags="-s -w" .

if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Go build failed" -ForegroundColor Red
    Pop-Location
    exit 1
}

if (-not (Test-Path "backend")) {
    Write-Host "Error: Backend binary not created" -ForegroundColor Red
    Pop-Location
    exit 1
}

$binarySize = (Get-Item "backend").Length / 1MB
Write-Host "Backend compiled successfully" -ForegroundColor Green
Write-Host "  Binary size: $([math]::Round($binarySize, 2)) MB"

Pop-Location

# ==========================================
# Deploy to EC2
# ==========================================
if ($KeyFile -ne "") {
    Write-Host "Step 5: Deploying backend to EC2..." -ForegroundColor Green
    
    if (-not (Test-Path $KeyFile)) {
        Write-Host "Error: Key file not found: $KeyFile" -ForegroundColor Red
        exit 1
    }
    
    $ecUser = "ec2-user"
    $remoteDir = "/opt/webapp"
    
    Write-Host "Testing SSH connection..." -ForegroundColor Yellow
    ssh -i $KeyFile -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$ecUser@$backendIp" "echo 'Connection successful'"
    
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Failed to connect to EC2 instance" -ForegroundColor Red
        exit 1
    }

    # Stop the service if it's running
    Write-Host "Stopping backend service..."
    ssh -i $KeyFile -o StrictHostKeyChecking=no ec2-user@$backendIp "sudo systemctl stop backend || true"

    Write-Host "Creating application directory..."
    ssh -i $KeyFile -o StrictHostKeyChecking=no "$ecUser@$backendIp" "sudo mkdir -p $remoteDir && sudo chown ec2-user:ec2-user $remoteDir"

    Write-Host "Uploading backend binary..." -ForegroundColor Yellow
    scp -i $KeyFile "$BackendDir/backend" "$ecUser@${backendIp}:$remoteDir/"
    ssh -i $KeyFile -o StrictHostKeyChecking=no "$ecUser@$backendIp" "chmod +x $remoteDir/backend"
    #ssh -i $KeyFile -o StrictHostKeyChecking=no "$ecUser@$backendIp" "/usr/bin/ls -l $remoteDir"
    
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Failed to upload backend binary" -ForegroundColor Red
        exit 1
    }

    Write-Host "Setting up backend service..."
    ssh -i $KeyFile -o StrictHostKeyChecking=no ec2-user@$backendIp @"
    sudo tee /etc/systemd/system/backend.service > /dev/null <<'EOF'
[Unit]
Description=Backend API Service
After=network.target

[Service]
Type=simple
User=ec2-user
WorkingDirectory=/opt/webapp
ExecStart=/opt/webapp/backend
Restart=always
RestartSec=5
Environment='PORT=8080'

StandardOutput=journal
StandardError=journal
SyslogIdentifier=backend

[Install]
WantedBy=multi-user.target
EOF
    sudo systemctl daemon-reload
    sudo systemctl enable backend
    sudo systemctl restart backend
"@

    Write-Host "Checking backend service status..."
    ssh -i $KeyFile -o StrictHostKeyChecking=no ec2-user@$backendIp "sudo systemctl status backend --no-pager"    
 
    Write-Host "restarting service..." -ForegroundColor Yellow
    ssh -i $KeyFile "$ecUser@$backendIp" @"

# Wait for service to start
Start-Sleep -Seconds 3

# Check service status
sudo systemctl status backend --no-pager

# Test health endpoint
Write-Host ""
Write-Host "Testing health endpoint..."
curl -f http://localhost:8080/api/health
"@

    Write-Host "Backend deployed and running" -ForegroundColor Green
} else {
    Write-Host "Step 5: Skipping EC2 deployment (no key file provided)" -ForegroundColor Yellow
    Write-Host "Backend binary available at: $BackendDir\backend" -ForegroundColor Yellow
    Write-Host "To deploy manually:" -ForegroundColor Yellow
    Write-Host "  scp -i YOUR_KEY backend ec2-user@{$backendIp}:/opt/webapp/"
    Write-Host "  ssh -i YOUR_KEY ec2-user@{$backendIp} 'sudo systemctl restart backend'"
}

# ==========================================
# Summary
# ==========================================
Write-Host ""
Write-Host "============================================" -ForegroundColor Green
Write-Host "           Deployment Complete!            " -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green
Write-Host ""
Write-Host "Backend:" -ForegroundColor Blue
Write-Host "  EC2 Instance: $backendIp"
Write-Host "  Load Balancer: http://$albEndpoint"
Write-Host ""

if ($KeyFile -ne "") {
    Write-Host "Useful Commands:" -ForegroundColor Blue
    Write-Host "  View logs:    ssh -i $KeyFile ec2-user@$backendIp 'sudo journalctl -u backend -f'"
    Write-Host "  Restart API:  ssh -i $KeyFile ec2-user@$backendIp 'sudo systemctl restart backend'"
    Write-Host "  Check status: ssh -i $KeyFile ec2-user@$backendIp 'sudo systemctl status backend'"
}

Write-Host ""
Write-Host "Your IT-gov-svc application is live!" -ForegroundColor Green