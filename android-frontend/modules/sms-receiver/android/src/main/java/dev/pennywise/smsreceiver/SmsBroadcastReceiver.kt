package dev.pennywise.smsreceiver

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.provider.Telephony
import android.util.Log
import com.facebook.react.HeadlessJsTaskService

/**
 * Declared in the manifest, so Android wakes the app for every incoming SMS
 * even when it is not running. SMS_RECEIVED is exempt from the implicit
 * broadcast limits and puts the app on the temporary allowlist, which is what
 * lets it start a background service from here.
 */
class SmsBroadcastReceiver : BroadcastReceiver() {
  override fun onReceive(context: Context, intent: Intent) {
    if (intent.action != Telephony.Sms.Intents.SMS_RECEIVED_ACTION) return
    val pdus = Telephony.Sms.Intents.getMessagesFromIntent(intent) ?: return

    // A long SMS arrives as several PDUs in one broadcast; join them per sender.
    pdus.filterNotNull().groupBy { it.displayOriginatingAddress.orEmpty() }.forEach { (sender, parts) ->
      val service = Intent(context, SmsHeadlessTaskService::class.java).apply {
        putExtra(SmsHeadlessTaskService.EXTRA_SENDER, sender)
        putExtra(SmsHeadlessTaskService.EXTRA_BODY, parts.joinToString("") { it.displayMessageBody.orEmpty() })
        putExtra(SmsHeadlessTaskService.EXTRA_TIMESTAMP, parts.first().timestampMillis)
      }
      try {
        context.startService(service)
        HeadlessJsTaskService.acquireWakeLockNow(context)
      } catch (e: IllegalStateException) {
        Log.w(TAG, "could not start SMS task service", e)
      }
    }
  }

  private companion object {
    const val TAG = "SmsReceiver"
  }
}
