#!/usr/bin/env python3
"""Run egress/install source against fake commands and isolated files; never touch firewall/systemd."""
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
from unittest.mock import patch
import sys

policy=Path(sys.argv[1]).read_text()
with tempfile.TemporaryDirectory(prefix='sub2api-egress-fixture-') as temp:
    root=Path(temp)
    cfg=root/'egress.json'
    cfg.write_text(json.dumps({'app_addresses':['172.30.253.2','172.30.254.2'],'database_addresses':['172.30.254.3'],'redis_addresses':['172.30.254.4']}))
    calls=[]
    def fake_run(args, **kw):
        calls.append((args,kw))
        assert args[0] in ('iptables','iptables-restore','ip6tables')
        return SimpleNamespace(returncode=int('-C' in args),stdout='',stderr='')
    with patch('subprocess.run',fake_run):
        exec(compile(policy.replace('/etc/sub2api-security/egress.json',str(cfg)),sys.argv[1],'exec'),{})
    rules=next(kw['input'] for args,kw in calls if args[0]=='iptables-restore')
    assert rules.index('-d 172.30.254.3 -p tcp --dport 5432 -j RETURN') < rules.index('-d 172.16.0.0/12 -j REJECT')
    assert '--dports 80,443,465,587' in rules
    for ip in ('172.30.253.2','172.30.254.2'):
        assert any(args[:5]==['iptables','-w','-I','DOCKER-USER','1'] and ip in args for args,kw in calls)
        assert any(args[:5]==['iptables','-w','-I','INPUT','1'] and ip in args and 'NEW' in args for args,kw in calls)
    assert any(args==['iptables','-w','-I','FORWARD','1','-j','DOCKER-USER'] for args,kw in calls)
    assert all(args[3]=='FORWARD' for args,kw in calls if args[0]=='ip6tables')
    print('PASS egress: fixed configuration, both app sources, host INPUT protection, database exceptions, FORWARD hook, IPv6 forwarding only')
