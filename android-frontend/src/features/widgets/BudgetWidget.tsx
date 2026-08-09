import { FlexWidget, TextWidget } from 'react-native-android-widget';
import { formatCurrency } from '../../utils/date';
import { Header, Shell, StatusCard } from './chrome';
import { widgetColors as c } from './widgetTheme';
import type { BudgetWidgetState } from './widgetData';

export const BUDGET_WIDGET_NAME = 'Budget';

function Stat({ label, value, tone }: { label: string; value: string; tone?: `#${string}` }) {
  return (
    <FlexWidget style={{ flexDirection: 'column', alignItems: 'flex-start' }}>
      <TextWidget text={label} style={{ fontSize: 10, color: c.faint }} />
      <TextWidget text={value} style={{ fontSize: 13, fontWeight: '600', color: tone ?? c.text, marginTop: 1 }} />
    </FlexWidget>
  );
}

/**
 * Zero-based budgeting's headline number: what is left to assign this month.
 * Zero is the goal, so an assigned-out budget reads as success rather than
 * emptiness.
 */
export function BudgetWidget({ state }: { state: BudgetWidgetState }) {
  if (state.kind !== 'ready') return <StatusCard title="Budget" state={state} />;

  const assignedOut = state.readyToAssign === 0;
  const overAssigned = state.readyToAssign < 0;

  const headlineTone = overAssigned ? c.danger : assignedOut ? c.success : c.primary;
  const trailing = overAssigned
    ? 'over-assigned'
    : assignedOut
      ? 'all assigned'
      : state.overspentCount > 0
        ? `${state.overspentCount} overspent`
        : undefined;
  const trailingTone = overAssigned ? c.danger : assignedOut ? c.success : c.warning;

  return (
    <Shell>
      <Header title="Budget" trailing={trailing} trailingColor={trailingTone} />

      <FlexWidget style={{ flexDirection: 'column', width: 'match_parent' }}>
        <TextWidget
          text={assignedOut ? 'All assigned' : 'Ready to assign'}
          style={{ fontSize: 11, color: c.muted }}
        />
        <TextWidget
          text={formatCurrency(state.readyToAssign)}
          maxLines={1}
          style={{ fontSize: 30, fontWeight: 'bold', color: headlineTone, adjustsFontSizeToFit: true }}
        />
      </FlexWidget>

      <FlexWidget
        style={{
          flexDirection: 'row',
          width: 'match_parent',
          justifyContent: 'space-between',
          alignItems: 'center'
        }}
      >
        <Stat label="Assigned" value={formatCurrency(state.assigned)} />
        <Stat label="Spent" value={formatCurrency(state.spent)} tone={c.warning} />
        <Stat label="Available" value={formatCurrency(state.available)} tone={c.success} />
      </FlexWidget>
    </Shell>
  );
}
