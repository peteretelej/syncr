# Running syncr as a Service

Run `syncr daemon` in the background so it starts automatically and survives reboots.

## Linux (systemd)

Create `/etc/systemd/system/syncr.service`:

```ini
[Unit]
Description=syncr - bidirectional folder sync
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=YOUR_USERNAME
ExecStart=/usr/local/bin/syncr -config /home/YOUR_USERNAME/syncr.json daemon
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Replace `YOUR_USERNAME` and paths as needed, then enable:

```bash
sudo systemctl daemon-reload
sudo systemctl enable syncr
sudo systemctl start syncr
```

Check status and logs:

```bash
sudo systemctl status syncr
journalctl -u syncr -f
```

## macOS (launchd)

Create `~/Library/LaunchAgents/com.syncr.daemon.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.syncr.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/syncr</string>
        <string>-config</string>
        <string>/Users/YOUR_USERNAME/syncr.json</string>
        <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/syncr.stdout.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/syncr.stderr.log</string>
</dict>
</plist>
```

Replace paths, then load:

```bash
launchctl load ~/Library/LaunchAgents/com.syncr.daemon.plist
```

Check status:

```bash
launchctl list | grep syncr
```

To stop and unload:

```bash
launchctl unload ~/Library/LaunchAgents/com.syncr.daemon.plist
```

## Windows (Task Scheduler)

1. Open **Task Scheduler** (search "Task Scheduler" in Start)
2. Click **Create Basic Task**
3. Name: `syncr daemon`
4. Trigger: **When the computer starts**
5. Action: **Start a program**
   - Program: `C:\path\to\syncr.exe`
   - Arguments: `-config C:\Users\YOUR_USERNAME\syncr.json daemon`
   - Start in: `C:\Users\YOUR_USERNAME`
6. Finish, then open the task properties and check:
   - **Run whether user is logged on or not**
   - **Run with highest privileges** (if syncing to protected paths)
   - Under Settings, uncheck **Stop the task if it runs longer than**

### Alternative: Windows Service with NSSM

[NSSM](https://nssm.cc/) wraps any executable as a proper Windows service.

```powershell
# Install NSSM (via chocolatey or download from nssm.cc)
choco install nssm

# Create the service
nssm install syncr "C:\path\to\syncr.exe"
nssm set syncr AppParameters "-config C:\Users\YOUR_USERNAME\syncr.json daemon"
nssm set syncr AppDirectory "C:\Users\YOUR_USERNAME"

# Start it
nssm start syncr
```

Manage with standard service commands:

```powershell
nssm status syncr
nssm stop syncr
nssm remove syncr confirm
```

## Notes

- syncr handles `SIGINT` and `SIGTERM` for graceful shutdown on Linux/macOS.
- Logs are written to `{sync_root}/_syncr/logs/` regardless of how the daemon is started.
- Config changes are picked up automatically without restarting the service.
- Use `syncr status` from the command line to check sync state at any time.
