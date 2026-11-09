import { requireOptionalNativeModule } from 'expo';

export type InboxMessage = { sender: string | null; body: string | null; timestamp: number };

type SmsReceiverModule = {
  /** Needs READ_SMS. Newest `limit` messages after `sinceMillis`, oldest first. */
  getInboxMessages(sinceMillis: number, limit: number): Promise<InboxMessage[]>;
};

// Optional, not required: this is imported at startup (via Settings), and a
// build without the native side must still open. Null means the APK lacks it.
export default requireOptionalNativeModule<SmsReceiverModule>('SmsReceiver');
