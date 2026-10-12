package com.zyrln.relay;

import android.app.Activity;
import android.content.Intent;
import android.net.VpnService;
import android.os.Build;
import android.os.Bundle;

/** Separate emulator test APK; uses only framework classes so it runs independently. */
public class LifecycleVpnFixtureActivity extends Activity {
    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        if (!Build.FINGERPRINT.contains("generic") && !Build.MODEL.contains("sdk")) {
            throw new IllegalStateException("Emulator fixture only");
        }
        if (getIntent().getBooleanExtra("stop_fixture", false)) {
            stopService(new Intent(this, LifecycleVpnFixtureService.class));
            finish();
            return;
        }
        Intent permission = VpnService.prepare(this);
        if (permission == null) launch(); else startActivityForResult(permission, 1);
    }
    @Override public void onActivityResult(int request, int result, Intent data) {
        super.onActivityResult(request, result, data);
        if (request == 1 && result == RESULT_OK) launch(); else finish();
    }
    private void launch() {
        startForegroundService(new Intent(this, LifecycleVpnFixtureService.class));
        finish();
    }
}
