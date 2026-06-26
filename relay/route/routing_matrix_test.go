package route

import "testing"

// Hosts used in routing matrix tests (domestic / Google / relay).
const (
	hostDigikala = "digikala.com"
	hostGoogle   = "www.google.com"
	hostYouTube  = "www.youtube.com"
)

func TestRoutingMatrix_ConnectRoute(t *testing.T) {
	loadBundledDomesticRules()
	if !ShouldUseDomesticBypass(hostDigikala) {
		t.Fatal("digikala.com must match domestic bundled list")
	}

	tests := []struct {
		name      string
		directOn  bool
		host      string
		wantRoute ConnectRoute
	}{
		// Android tunnel + direct ON, Desktop relay + direct ON
		{"tunnel/relay direct ON: digikala", true, hostDigikala, RouteDomesticPlain},
		{"tunnel/relay direct ON: google", true, hostGoogle, RouteDirectFragment},
		{"tunnel/relay direct ON: youtube", true, hostYouTube, RouteRelay},

		// Android tunnel + direct OFF, Desktop relay + direct OFF (google differs)
		{"tunnel/relay direct OFF: digikala", false, hostDigikala, RouteDomesticPlain},
		{"tunnel/relay direct OFF: google", false, hostGoogle, RouteRelay},
		{"tunnel/relay direct OFF: youtube", false, hostYouTube, RouteRelay},

		// Direct-only: Google vs everything else (same host routes as direct ON above)
		{"direct only: digikala", true, hostDigikala, RouteDomesticPlain},
		{"direct only: google", true, hostGoogle, RouteDirectFragment},
		{"direct only: youtube", true, hostYouTube, RouteRelay},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			SetDirectEnabled(tc.directOn)
			got := ConnectRouteForHost(tc.host)
			if got != tc.wantRoute {
				t.Fatalf("ConnectRouteForHost(%q) = %v, want %v", tc.host, got, tc.wantRoute)
			}
		})
	}
}

func TestRoutingMatrix_GoogleNeverDomestic(t *testing.T) {
	loadBundledDomesticRules()
	SetDirectEnabled(true)
	if ShouldUseDomesticBypass(hostGoogle) {
		t.Fatal("google must not use domestic bypass")
	}
}
