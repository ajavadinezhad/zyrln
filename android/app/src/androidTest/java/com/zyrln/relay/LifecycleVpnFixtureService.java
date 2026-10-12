package com.zyrln.relay;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Intent;
import android.net.VpnService;
import android.os.Build;
import android.os.ParcelFileDescriptor;
import java.io.IOException;

public class LifecycleVpnFixtureService extends VpnService {
    private ParcelFileDescriptor descriptor;
    @Override public int onStartCommand(Intent intent, int flags, int id) {
        if (!Build.FINGERPRINT.contains("generic") && !Build.MODEL.contains("sdk")) {
            throw new IllegalStateException("Emulator fixture only");
        }
        String channel = "zyrln_lifecycle_fixture";
        getSystemService(NotificationManager.class).createNotificationChannel(
            new NotificationChannel(channel, "Emulator VPN test", NotificationManager.IMPORTANCE_LOW));
        startForeground(10, new Notification.Builder(this, channel).setContentTitle("Emulator VPN test")
            .setSmallIcon(android.R.drawable.ic_dialog_info).build());
        descriptor = new Builder().setSession("Zyrln lifecycle fixture").addAddress("10.88.0.1", 32)
            .addRoute("192.0.2.0", 24).establish();
        if (descriptor == null) stopSelf();
        return START_NOT_STICKY;
    }
    @Override public void onDestroy() {
        if (descriptor != null) {
            try { descriptor.close(); } catch (IOException ignored) { }
            descriptor = null;
        }
        stopForeground(STOP_FOREGROUND_REMOVE);
        super.onDestroy();
    }
}
