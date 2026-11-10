import type { WidgetTaskHandlerProps } from 'react-native-android-widget';
import { StatusCard } from './chrome';
import { WIDGET_RENDERERS } from './renderers';

/**
 * Entry point Android calls for every widget lifecycle event, for every widget.
 * Runs headlessly: no Redux store, no React tree, no app UI -- which is why
 * data access goes through `utils/headlessApi` rather than `utils/api`.
 */
export async function widgetTaskHandler(props: WidgetTaskHandlerProps): Promise<void> {
  const name = props.widgetInfo.widgetName;

  try {
    if (props.widgetAction === 'WIDGET_DELETED') return;

    // Every widget is render-only, so a click is a plain redraw.
    const render = WIDGET_RENDERERS[name];
    if (!render) {
      // app.config.ts and the renderer map disagree, or an older APK still has a
      // removed widget on the home screen.
      console.log('[widget] no renderer registered for', name);
      return;
    }
    props.renderWidget(await render());
  } catch (error) {
    // Throwing here would leave whatever was last drawn on the home screen,
    // which is worse than showing the failure.
    console.log(`[widget] ${name} handler failed`, error);
    const message = error instanceof Error ? error.message : 'Unexpected error';
    props.renderWidget(<StatusCard title={name} state={{ kind: 'error', message }} />);
  }
}
