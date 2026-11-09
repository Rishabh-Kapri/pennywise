package dev.pennywise.smsreceiver

import android.content.Intent
import com.facebook.react.HeadlessJsTaskService
import com.facebook.react.bridge.Arguments
import com.facebook.react.jstasks.HeadlessJsTaskConfig

/** Runs the JS task registered under [TASK_KEY] in `index.js` for one SMS. */
class SmsHeadlessTaskService : HeadlessJsTaskService() {
  override fun getTaskConfig(intent: Intent?): HeadlessJsTaskConfig? {
    val extras = intent?.extras ?: return null
    val data = Arguments.createMap().apply {
      putString("sender", extras.getString(EXTRA_SENDER))
      putString("body", extras.getString(EXTRA_BODY))
      putDouble("timestamp", extras.getLong(EXTRA_TIMESTAMP).toDouble())
    }
    // Allowed in the foreground too: an SMS can arrive while the app is open.
    return HeadlessJsTaskConfig(TASK_KEY, data, TIMEOUT_MS, true)
  }

  companion object {
    const val TASK_KEY = "SmsReceived"
    const val EXTRA_SENDER = "sender"
    const val EXTRA_BODY = "body"
    const val EXTRA_TIMESTAMP = "timestamp"
    private const val TIMEOUT_MS = 30_000L
  }
}
