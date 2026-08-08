import type { WidgetTaskHandlerProps } from 'react-native-android-widget';
import { ACCOUNTS_WIDGET_NAME, AccountsWidget } from './AccountsWidget';
import { BUDGET_WIDGET_NAME, BudgetWidget } from './BudgetWidget';
import { PIPELINE_WIDGET_NAME, PipelineWidget, RETRY_PARKED_ACTION } from './PipelineWidget';
import { RECENT_WIDGET_NAME, RecentWidget } from './RecentWidget';
import { StatusCard } from './chrome';
import { loadPipelineState, retryParkedRuns } from './pipelineData';
import { loadAccountsState, loadBudgetState, loadRecentState } from './widgetData';

/**
 * Entry point Android calls for every widget lifecycle event, for every widget.
 * Runs headlessly: no Redux store, no React tree, no app UI -- which is why
 * data access goes through `utils/headlessApi` rather than `utils/api`.
 */
export async function widgetTaskHandler(props: WidgetTaskHandlerProps): Promise<void> {
  const name = props.widgetInfo.widgetName;

  try {
    if (props.widgetAction === 'WIDGET_DELETED') return;

    // The only interactive widget today. Everything else is render-only, so a
    // click falls through to a plain re-render.
    if (props.widgetAction === 'WIDGET_CLICK') {
      if (name === PIPELINE_WIDGET_NAME && props.clickAction === RETRY_PARKED_ACTION) {
        await handleRetry(props);
      }
      return;
    }

    await render(props, name);
  } catch (error) {
    // Throwing here would leave whatever was last rendered on the home screen,
    // which is worse than showing the failure.
    console.log(`[widget] ${name} handler failed`, error);
    const message = error instanceof Error ? error.message : 'Unexpected error';
    props.renderWidget(<StatusCard title={name} state={{ kind: 'error', message }} />);
  }
}

async function render(props: WidgetTaskHandlerProps, name: string): Promise<void> {
  switch (name) {
    case PIPELINE_WIDGET_NAME:
      props.renderWidget(<PipelineWidget state={await loadPipelineState()} />);
      return;
    case BUDGET_WIDGET_NAME:
      props.renderWidget(<BudgetWidget state={await loadBudgetState()} />);
      return;
    case ACCOUNTS_WIDGET_NAME:
      props.renderWidget(<AccountsWidget state={await loadAccountsState()} />);
      return;
    case RECENT_WIDGET_NAME:
      props.renderWidget(<RecentWidget state={await loadRecentState()} />);
      return;
    default:
      // An unknown name means the config plugin and this switch disagree.
      console.log('[widget] no renderer registered for', name);
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
