import { requestWidgetUpdate } from 'react-native-android-widget';
import { WIDGET_NAMES, WIDGET_RENDERERS } from './renderers';

/**
 * Redraws widgets on demand, so a purchase shows up on the home screen straight
 * away instead of waiting on Android's 30-minute `updatePeriodMillis` floor.
 * That floor is the fallback; this is the real freshness mechanism.
 *
 * Safe to call from anywhere, including headless contexts. The callback only
 * runs for widgets actually placed on a home screen, so this costs nothing when
 * none are added.
 */
export async function refreshWidgets(names: string[] = WIDGET_NAMES): Promise<void> {
  await Promise.all(
    names.map(async (widgetName) => {
      const render = WIDGET_RENDERERS[widgetName];
      if (!render) return;
      try {
        await requestWidgetUpdate({
          widgetName,
          renderWidget: () => render()
        });
      } catch (error) {
        // Never let a widget redraw break the caller: this runs as a side
        // effect of saving a transaction, and the save has already succeeded.
        console.log(`[widget] refresh failed for ${widgetName}`, error);
      }
    })
  );
}

/**
 * Widgets whose contents depend on transactions or budget assignments. Used to
 * avoid redrawing unrelated widgets (ingestion health) on every edit.
 */
export const SPEND_SENSITIVE_WIDGETS = ['Pressure', 'Budget', 'Accounts', 'Recent'];
