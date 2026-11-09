import { registerRootComponent } from 'expo';
import { AppRegistry } from 'react-native';
import { registerWidgetTaskHandler } from 'react-native-android-widget';

import App from './App';
import { widgetTaskHandler } from './src/features/widgets/widgetTaskHandler';
import { SMS_TASK_KEY, smsTask } from './src/features/sms/smsForwarder';

registerRootComponent(App);

// Registered at the entry point rather than inside a component: Android starts
// this bundle headlessly to service widget updates, with no app UI mounted.
registerWidgetTaskHandler(widgetTaskHandler);

// Same reason: modules/sms-receiver starts this headlessly for each incoming SMS.
AppRegistry.registerHeadlessTask(SMS_TASK_KEY, () => smsTask);
