import { ACCOUNTS_WIDGET_NAME, AccountsWidget } from './AccountsWidget';
import { BUDGET_WIDGET_NAME, BudgetWidget } from './BudgetWidget';
import { PIPELINE_WIDGET_NAME, PipelineWidget } from './PipelineWidget';
import { PRESSURE_WIDGET_NAME, PressureWidget } from './PressureWidget';
import { RECENT_WIDGET_NAME, RecentWidget } from './RecentWidget';
import { loadPipelineState } from './pipelineData';
import { loadAccountsState, loadBudgetState, loadPressureState, loadRecentState } from './widgetData';

/**
 * Single source of truth for "how do I draw widget X right now".
 *
 * Both entry points use it -- the Android task handler (widget added, periodic
 * update, resize) and the in-app refresh that fires after data changes -- so
 * there is one place a new widget has to be registered rather than two that can
 * drift. The keys must match the `name` fields in `app.config.ts`.
 */
export const WIDGET_RENDERERS: Record<string, () => Promise<React.JSX.Element>> = {
  [PRESSURE_WIDGET_NAME]: async () => <PressureWidget state={await loadPressureState()} />,
  [BUDGET_WIDGET_NAME]: async () => <BudgetWidget state={await loadBudgetState()} />,
  [ACCOUNTS_WIDGET_NAME]: async () => <AccountsWidget state={await loadAccountsState()} />,
  [RECENT_WIDGET_NAME]: async () => <RecentWidget state={await loadRecentState()} />,
  [PIPELINE_WIDGET_NAME]: async () => <PipelineWidget state={await loadPipelineState()} />
};

export const WIDGET_NAMES = Object.keys(WIDGET_RENDERERS);
