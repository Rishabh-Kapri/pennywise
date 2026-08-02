import { registerRootComponent } from 'expo';
import { registerWidgetTaskHandler } from 'react-native-android-widget';

import App from './App';
import { widgetTaskHandler } from './src/features/widgets/widgetTaskHandler';

registerRootComponent(App);

// Registered at the entry point rather than inside a component: Android starts
// this bundle headlessly to service widget updates, with no app UI mounted.
registerWidgetTaskHandler(widgetTaskHandler);
