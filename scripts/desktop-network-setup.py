"""Dedicated namespace; no host routes, forwarding, firewall, or adapter changes."""
import json,os,pwd,shutil,socket,subprocess,time
from pathlib import Path
name='zyrln-desktop-test'
root=Path('/home/aj/.config/zyrln-tests/desktop-network')
root.mkdir(mode=0o700,exist_ok=True)
owner=pwd.getpwnam('aj');os.chown(root,owner.pw_uid,owner.pw_gid)
subprocess.run(['ip','netns','add',name],check=True)
dns=Path('/etc/netns')/name;dns.mkdir(parents=True,exist_ok=False)
(dns/'resolv.conf').write_text('nameserver 10.0.2.3\n')
sock=root/'slirp.sock'
log=(root/'namespace.log').open('ab')
slirp=subprocess.Popen(['slirp4netns','--configure','--disable-host-loopback','--netns-type=path','--api-socket='+str(sock),'/var/run/netns/'+name,'tap0'],stdout=log,stderr=log,start_new_session=True)
(root/'slirp.pid').write_text(str(slirp.pid))
for _ in range(100):
 if sock.exists():break
 time.sleep(.1)
else:raise RuntimeError('Network helper did not start')
for port in [18096,18095,11090]:
 s=socket.socket(socket.AF_UNIX);s.connect(str(sock))
 s.sendall(json.dumps({'execute':'add_hostfwd','arguments':{'proto':'tcp','host_addr':'127.0.0.1','host_port':port,'guest_addr':'10.0.2.100','guest_port':port}}).encode());s.shutdown(socket.SHUT_WR)
 response=json.loads(s.recv(4096));s.close()
 if 'error' in response:raise RuntimeError('Host forward failed')
source=Path('/home/aj/.config/zyrln-tests/desktop/config.env')
lines=source.read_text().splitlines()
for k,v in [('listen','0.0.0.0:18095'),('socks-listen','0.0.0.0:11090'),('direct-enabled','false')]:
 lines=[l for l in lines if not l.startswith(k+'=')];lines.append(k+'='+v)
config=root/'config.env';config.write_text('\n'.join(lines)+'\n');config.chmod(0o600);os.chown(config,owner.pw_uid,owner.pw_gid)
gui=subprocess.Popen(['ip','netns','exec',name,'runuser','-u','aj','--','/home/aj/.config/zyrln-tests/desktop/zyrln','-gui','-gui-listen','0.0.0.0:18096','-config',str(config),'-ca-cert',str(root/'ca.pem'),'-ca-key',str(root/'ca-key.pem')],stdout=log,stderr=log,start_new_session=True)
(root/'gui.pid').write_text(str(gui.pid))
print('Dedicated namespace and disposable GUI started')
