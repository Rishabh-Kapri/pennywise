package dev.pennywise.smsreceiver

import android.Manifest
import android.content.pm.PackageManager
import android.provider.Telephony
import androidx.core.content.ContextCompat
import expo.modules.kotlin.exception.CodedException
import expo.modules.kotlin.exception.Exceptions
import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.modules.ModuleDefinition

class ReadSmsPermissionException : CodedException("READ_SMS permission is not granted")

// Live SMS reach JS through the headless task (SmsBroadcastReceiver); this
// module only serves reads of the existing inbox.
class SmsReceiverModule : Module() {
  override fun definition() = ModuleDefinition {
    Name("SmsReceiver")

    // Inbox messages received after `sinceMillis`, newest `limit` of them,
    // returned oldest first.
    AsyncFunction("getInboxMessages") { sinceMillis: Double, limit: Int ->
      val context = appContext.reactContext ?: throw Exceptions.ReactContextLost()
      if (ContextCompat.checkSelfPermission(context, Manifest.permission.READ_SMS) != PackageManager.PERMISSION_GRANTED) {
        throw ReadSmsPermissionException()
      }

      val messages = mutableListOf<Map<String, Any?>>()
      context.contentResolver.query(
        Telephony.Sms.Inbox.CONTENT_URI,
        arrayOf(
          Telephony.TextBasedSmsColumns.ADDRESS,
          Telephony.TextBasedSmsColumns.BODY,
          Telephony.TextBasedSmsColumns.DATE,
          Telephony.TextBasedSmsColumns.DATE_SENT
        ),
        "${Telephony.TextBasedSmsColumns.DATE} > ?",
        arrayOf(sinceMillis.toLong().toString()),
        "${Telephony.TextBasedSmsColumns.DATE} DESC"
      )?.use { cursor ->
        while (messages.size < limit && cursor.moveToNext()) {
          messages.add(
            mapOf(
              "sender" to cursor.getString(0),
              "body" to cursor.getString(1),
              // DATE_SENT is the time a live SMS carries, and ingest dedupes on it.
              // DATE is when the phone stored the message.
              "timestamp" to (cursor.getLong(3).takeIf { it > 0 } ?: cursor.getLong(2)).toDouble()
            )
          )
        }
      }
      messages.reversed()
    }
  }
}
