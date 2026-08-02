import type { WidgetTaskHandlerProps } from 'react-native-android-widget';
import { PIPELINE_WIDGET_NAME, PipelineWidget, RETRY_PARKED_ACTION } from './PipelineWidget';
import { loadPipelineState, retryParkedRuns } from './pipelineData';

/**
 * Entry point Android calls for every widget lifecycle event. Runs headlessly:
 * no Redux store, no React tree, no app UI -- which is why data access goes
 * through `utils/headlessApi` rather than `utils/api`.
 */
export async function widgetTaskHandler(props: WidgetTaskHandlerProps): Promise<void> {
  if (props.widgetInfo.widgetName !== PIPELINE_WIDGET_NAME) return;

  const render = async () => {
    props.renderWidget(<PipelineWidget state={await loadPipelineState()} />);
  };

  try {
    switch (props.widgetAction) {
      case 'WIDGET_ADDED':
      case 'WIDGET_UPDATE':
      case 'WIDGET_RESIZED':
        await render();
        break;

      case 'WIDGET_CLICK':
        if (props.clickAction !== RETRY_PARKED_ACTION) break;
        await handleRetry(props);
        break;

      case 'WIDGET_DELETED':
        break;
    }
  } catch (error) {
    // A throw here leaves whatever was last rendered on the home screen, which
    // is worse than showing the failure.
    console.log('[widget] handler failed', error);
    props.renderWidget(
      <PipelineWidget
        state={{ kind: 'error', message: error instanceof Error ? error.message : 'Unexpected error' }}
      />
    );
  }
}

async function handleRetry(props: WidgetTaskHandlerProps): Promise<void> {
  const current = await loadPipelineState();
  if (current.kind !== 'ready' || current.parked.length === 0) {
    props.renderWidget(<PipelineWidget state={current} />);
    return;
  }

  // Each tap spins up a headless JS context, so the round trip is visible.
  // Paint the pending state first rather than leaving the old count on screen.
  props.renderWidget(<PipelineWidget state={{ kind: 'retrying', count: current.parked.length }} />);

  await retryParkedRuns(current.parked);

  // Re-read rather than assuming: a retry signals a Temporal workflow, so a run
  // may still be parked if the signal was accepted but the work has not moved.
  props.renderWidget(<PipelineWidget state={await loadPipelineState()} />);
}
