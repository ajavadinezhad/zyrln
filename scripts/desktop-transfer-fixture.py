import hashlib,json,time
from http.server import ThreadingHTTPServer,BaseHTTPRequestHandler
BLOCK=bytes(range(256))*256
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def send(self,body,kind='application/json'):
  self.send_response(200);self.send_header('Content-Type',kind);self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
 def do_GET(self):
  if self.path.startswith('/download/'):
   try:n=int(self.path.split('/')[2].split('?')[0])
   except: self.send_error(400);return
   if not 0<=n<=8*1024*1024:self.send_error(413);return
   self.send_response(200);self.send_header('Content-Type','application/octet-stream');self.send_header('Content-Length',str(n));self.end_headers()
   for off in range(0,n,len(BLOCK)):
    self.wfile.write(BLOCK[:min(len(BLOCK),n-off)]);self.wfile.flush();time.sleep(.02)
  else:self.send_error(404)
 def do_POST(self):
  if self.path!='/upload':self.send_error(404);return
  n=int(self.headers.get('Content-Length','0'))
  if not 0<=n<=8*1024*1024:self.send_error(413);return
  h=hashlib.sha256();count=0
  while count<n:
   b=self.rfile.read(min(65536,n-count))
   if not b:break
   count+=len(b);h.update(b)
  self.send(json.dumps({'bytes':count,'sha256':h.hexdigest()}).encode())
ThreadingHTTPServer(('0.0.0.0',8788),Handler).serve_forever()