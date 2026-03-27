# fail2ban Integration

LM Gate logs authentication failures and rate-limit events to stdout with a `[SECURITY]` prefix. This makes it straightforward to integrate with [fail2ban](https://github.com/fail2ban/fail2ban) to automatically ban IPs that repeatedly trigger these events.

## Log Format

```
[SECURITY] 2024-01-15T10:30:45Z 401 POST /v1/chat/completions ip=192.168.1.1
[SECURITY] 2024-01-15T10:30:46Z 429 POST /v1/chat/completions ip=192.168.1.1 user=alice@example.com
```

- `401` lines are auth failures (bad/missing token)
- `429` lines are rate-limit violations (may include the authenticated user)

## Setup

### 1. Route LM Gate logs to a file

fail2ban reads log files, so you need to persist LM Gate's stdout to a file.

**Docker (recommended):** Configure the container's logging driver to write to a file. Add to your `docker-compose*.yml`:

```yaml
services:
  lmgate:
    # ... existing config ...
    logging:
      driver: json-file
      options:
        max-size: "50m"
        max-file: "3"
```

Docker writes these logs to `/var/lib/docker/containers/<container-id>/<container-id>-json.log`. To get your container's log path:

```bash
docker inspect --format='{{.LogPath}}' lmgate
```

**Binary (systemd):** If running LM Gate as a systemd service, journald already captures stdout. You can either point fail2ban at the journald backend or redirect output to a file:

```ini
# In your systemd unit file
StandardOutput=append:/var/log/lmgate.log
StandardError=append:/var/log/lmgate.log
```

### 2. Create the fail2ban filter

Create `/etc/fail2ban/filter.d/lmgate.conf`:

```ini
[Definition]
# Match auth failures (401) and rate-limit events (429)
failregex = ^\{"log":".*\[SECURITY\] \S+ (?:401|429) \S+ \S+ ip=<HOST>
            ^.*\[SECURITY\] \S+ (?:401|429) \S+ \S+ ip=<HOST>

ignoreregex =
```

The first pattern matches Docker json-file log format. The second matches plain stdout (systemd/file redirect).

### 3. Create the fail2ban jail

Create `/etc/fail2ban/jail.d/lmgate.conf`:

```ini
[lmgate]
enabled  = true
filter   = lmgate
port     = http,https
# Point to your actual log path (see step 1)
logpath  = /var/lib/docker/containers/<container-id>/<container-id>-json.log
maxretry = 5
findtime = 300
bantime  = 3600
```

| Setting    | Description                                       |
|------------|---------------------------------------------------|
| `maxretry` | Number of failures before banning (default: 5)    |
| `findtime` | Window in seconds to count failures (default: 300) |
| `bantime`  | Ban duration in seconds (default: 3600 = 1 hour)  |

Adjust these values based on your security requirements. For more aggressive protection, lower `maxretry` or increase `bantime`.

### 4. Restart fail2ban

```bash
sudo systemctl restart fail2ban
```

### 5. Verify

Check that the jail is active:

```bash
sudo fail2ban-client status lmgate
```

Test the filter against your log file:

```bash
sudo fail2ban-regex /path/to/lmgate.log /etc/fail2ban/filter.d/lmgate.conf
```

## Useful Commands

```bash
# Check banned IPs
sudo fail2ban-client status lmgate

# Manually unban an IP
sudo fail2ban-client set lmgate unbanip 192.168.1.1

# Follow fail2ban's own log
sudo tail -f /var/log/fail2ban.log
```

## Notes

- The `[SECURITY]` log line is written synchronously to stdout so it is never lost, even if the async DB write is dropped due to a full channel.
- If LM Gate is behind a reverse proxy, make sure the proxy forwards the real client IP via `X-Real-IP` or `X-Forwarded-For` and that LM Gate is configured to trust the proxy. Otherwise, fail2ban will see the proxy's IP instead of the client's.
