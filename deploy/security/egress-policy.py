#!/usr/bin/python3
"""Container egress baseline. HTTPS destination allowlisting is a separate policy."""
import ipaddress
import json
from pathlib import Path
import subprocess

def run(*args, check=True):
    return subprocess.run(list(args), capture_output=True, text=True, check=check)

# Fixed Compose addresses allow enforcement before Docker starts containers.
config = json.loads(Path("/etc/sub2api-security/egress.json").read_text())
apps = config["app_addresses"]
databases = config["database_addresses"]
redis = config["redis_addresses"]
assert apps and databases and redis
for ip in apps + databases + redis:
    assert ipaddress.ip_address(ip).version == 4
chain = "SUB2API-EGRESS"
run("iptables", "-w", "-N", chain, check=False)
run("iptables", "-w", "-N", "DOCKER-USER", check=False)
lines = ["*filter", ":" + chain + " - [0:0]", "-F " + chain]
for ip in ("45.88.91.90", "154.219.115.131"):
    lines.append(f"-A {chain} -d {ip} -j REJECT")
lines.append(f"-A {chain} -m conntrack --ctstate ESTABLISHED,RELATED -j RETURN")
for ips, port in ((databases, 5432), (redis, 6379)):
    for ip in ips:
        ipaddress.ip_address(ip)
        lines.append(f"-A {chain} -d {ip} -p tcp --dport {port} -j RETURN")
# Docker's embedded resolver handles application DNS; do not permit metadata or
# arbitrary private destinations. This deployment uses public upstream/proxy hosts.
for subnet in ("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16"):
    lines.append(f"-A {chain} -d {subnet} -j REJECT")
lines.append(f"-A {chain} -p tcp -m multiport --dports 80,443,465,587 -j RETURN")
lines.append(f'-A {chain} -m limit --limit 6/min --limit-burst 10 -j LOG --log-prefix "SUB2API-EGRESS "')
lines.extend([f"-A {chain} -j REJECT", "COMMIT"])
subprocess.run(["iptables-restore", "--noflush", "-w"], input="\n".join(lines)+"\n", text=True, check=True)
for ip in apps:
    ipaddress.ip_address(ip)
    rule = ["-s", ip, "-j", chain]
    if run("iptables", "-w", "-C", "DOCKER-USER", *rule, check=False).returncode:
        run("iptables", "-w", "-I", "DOCKER-USER", "1", *rule)
    # Local host services follow INPUT, not DOCKER-USER. Application dependencies
    # live in containers or on public endpoints, so new host connections are denied.
    host_rule = ["-s", ip, "-m", "conntrack", "--ctstate", "NEW", "-j", "REJECT"]
    if run("iptables", "-w", "-C", "INPUT", *host_rule, check=False).returncode:
        run("iptables", "-w", "-I", "INPUT", "1", *host_rule)
if run("iptables", "-w", "-C", "FORWARD", "-j", "DOCKER-USER", check=False).returncode:
    run("iptables", "-w", "-I", "FORWARD", "1", "-j", "DOCKER-USER")
# These networks use IPv4. Block forwarded IPv6 from Docker bridges so enabling
# a container IPv6 address cannot bypass the IPv4 policy. Host IPv6 is untouched.
for interface in ("docker0", "br+"):
    rule = ["-i", interface, "-j", "REJECT"]
    if run("ip6tables", "-w", "-C", "FORWARD", *rule, check=False).returncode:
        run("ip6tables", "-w", "-I", "FORWARD", "1", *rule)
print("Applied container egress baseline and Docker IPv6 forwarding restriction")
