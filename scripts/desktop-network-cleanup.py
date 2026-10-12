"""Stop only the dedicated namespace's checked processes and remove its binding."""
import os,signal,subprocess
from pathlib import Path
root=Path('/home/aj/.config/zyrln-tests/desktop-network')
for filename,marker,group in [('gui.pid',b'/home/aj/.config/zyrln-tests/desktop/zyrln',True),('slirp.pid',b'/var/run/netns/zyrln-desktop-test',False)]:
 p=root/filename
 if not p.exists():continue
 pid=int(p.read_text());cmd=Path('/proc')/str(pid)/'cmdline'
 if cmd.exists() and marker in cmd.read_bytes():
  if group and os.getpgid(pid)==pid:os.killpg(pid,signal.SIGTERM)
  elif not group:os.kill(pid,signal.SIGTERM)
subprocess.run(['ip','netns','delete','zyrln-desktop-test'],check=True)
dns=Path('/etc/netns/zyrln-desktop-test')
if (dns/'resolv.conf').exists():(dns/'resolv.conf').unlink()
if dns.exists():dns.rmdir()
print('Dedicated test namespace, GUI and network helper stopped')
