Zyrln VPS installer

This package is for the owner of a Linux VPS. App users with an existing
config do not need to install it.

Full instructions:
https://github.com/ajavadinezhad/zyrln/blob/main/docs/relay-setup.md
Persian:
https://github.com/ajavadinezhad/zyrln/blob/main/docs/fa/relay-setup.md

From a Bash terminal in this extracted folder, replace the placeholders:

  chmod +x install-vps-relay.sh
  ZYRLN_RELAY_KEY='YOUR_EXIT_KEY' ./install-vps-relay.sh user@YOUR_VPS_IP

Use a long random exit key. Keep it private and enter the same value as
EXIT_RELAY_KEY in Google Apps Script. The app itself uses a separate app key.
The SSH user needs root or sudo access. Allow inbound TCP port 8787 in the
provider firewall. Both amd64 and arm64 server binaries are included.

Check the server: curl http://YOUR_VPS_IP:8787/healthz
Expected response: ok

Then publish the Google Apps Script bridge using the full guide. The app
must use the Apps Script URL ending in /exec, never the VPS address.
