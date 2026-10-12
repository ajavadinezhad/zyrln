package netdial_test

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"zyrln/relay/appscript"
	"zyrln/relay/netdial"
)

func TestUnderlyingResolutionRetainsTLSHostnameVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "verified") }))
	defer server.Close()
	cert := server.Certificate()
	if len(cert.DNSNames) == 0 {
		t.Fatal("fixture requires DNS certificate name")
	}
	_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	netdial.SetHostResolver(func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("127.0.0.1")}, nil })
	t.Cleanup(func() { netdial.SetHostResolver(nil) })
	client := appscript.NewHTTPClient(time.Second)
	defer client.CloseIdleConnections()
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	response, err := client.Get("https://" + net.JoinHostPort(cert.DNSNames[0], port))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	response, err = client.Get("https://" + net.JoinHostPort("wrong-host.invalid", port))
	if response != nil {
		response.Body.Close()
	}
	if err == nil {
		t.Fatal("wrong TLS hostname was accepted")
	}
}
