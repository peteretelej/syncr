# Run this script as Administrator to create a scheduled task for pm2 resurrect

# Get pm2 path
$pm2Path = (Get-Command pm2 -ErrorAction SilentlyContinue).Source
if (-not $pm2Path) {
    $pm2Path = "$env:APPDATA\npm\pm2.cmd"
}

Write-Host "Using pm2 at: $pm2Path"

# Create scheduled task
$action = New-ScheduledTaskAction -Execute $pm2Path -Argument "resurrect"
$trigger = New-ScheduledTaskTrigger -AtLogOn
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable

Register-ScheduledTask -TaskName "PM2 Resurrect" -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force

Write-Host "Scheduled task 'PM2 Resurrect' created successfully!"
Write-Host "It will run 'pm2 resurrect' at every logon."
Write-Host ""
Write-Host "Don't forget to save your pm2 processes:"
Write-Host "  pm2 save"
