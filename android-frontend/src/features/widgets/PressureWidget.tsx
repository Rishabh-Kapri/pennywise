import { FlexWidget, TextWidget } from 'react-native-android-widget';
import { formatCurrency } from '../../utils/date';
import { Header, Shell, StatusCard } from './chrome';
import { widgetColors as c } from './widgetTheme';
import type { CategoryPressure, PressureWidgetState } from './widgetData';

export const PRESSURE_WIDGET_NAME = 'Pressure';

const toneFor = (reason: CategoryPressure['reason']) =>
  reason === 'over' ? c.danger : reason === 'ahead' ? c.warning : c.primary;

/**
 * A progress bar, built from flex weights.
 *
 * RemoteViews has no percentage width, so the fill and the remainder are two
 * boxes sharing a row by weight. Weights must stay above zero or the child
 * collapses and Android lays the row out unpredictably.
 */
function Bar({ fraction, tone }: { fraction: number; tone: `#${string}` }) {
  const filled = Math.min(1, Math.max(0.02, Number.isFinite(fraction) ? fraction : 1));
  const rest = Math.max(0.02, 1 - filled);

  return (
    <FlexWidget
      style={{
        width: 'match_parent',
        height: 4,
        flexDirection: 'row',
        backgroundColor: c.surfaceStrong,
        borderRadius: 999,
        marginTop: 4
      }}
    >
      <FlexWidget style={{ flex: filled, height: 4, backgroundColor: tone, borderRadius: 999 }} />
      <FlexWidget style={{ flex: rest, height: 4 }} />
    </FlexWidget>
  );
}

function summaryFor(entry: CategoryPressure): string {
  if (entry.reason === 'over') {
    return `${formatCurrency(Math.abs(entry.remaining))} over`;
  }
  if (entry.reason === 'ahead') {
    // The projection is what makes this actionable: "on pace for X" says more
    // than a percentage, because it names the outcome if nothing changes.
    return `${formatCurrency(entry.remaining)} left · on pace for ${formatCurrency(entry.projected)}`;
  }
  return `${formatCurrency(entry.remaining)} left of ${formatCurrency(entry.budgeted)}`;
}

function Row({ entry }: { entry: CategoryPressure }) {
  const tone = toneFor(entry.reason);
  return (
    <FlexWidget style={{ width: 'match_parent', flexDirection: 'column', marginTop: 8 }}>
      <FlexWidget
        style={{
          width: 'match_parent',
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'center'
        }}
      >
        <TextWidget
          text={entry.name}
          maxLines={1}
          truncate="END"
          style={{ fontSize: 13, fontWeight: '600', color: c.text }}
        />
        <TextWidget
          text={entry.reason === 'over' ? 'over' : entry.reason === 'ahead' ? 'fast' : 'close'}
          style={{ fontSize: 10, fontWeight: '600', color: tone, marginLeft: 8 }}
        />
      </FlexWidget>

      <TextWidget
        text={summaryFor(entry)}
        maxLines={1}
        truncate="END"
        style={{ fontSize: 11, color: c.muted }}
      />
      <Bar fraction={entry.usedFraction} tone={tone} />
    </FlexWidget>
  );
}

/**
 * Surfaces the categories that need attention *now*, rather than a full budget
 * listing. Ranking is pace-aware: 90% spent on the 3rd of the month is a
 * different problem from 90% spent on the 28th, and only the first is worth
 * putting on someone's home screen.
 */
export function PressureWidget({ state }: { state: PressureWidgetState }) {
  if (state.kind !== 'ready') return <StatusCard title="Watch" state={state} />;

  if (state.pressured.length === 0) {
    return (
      <Shell>
        <Header title="Watch" trailing="on track" trailingColor={c.success} />
        <FlexWidget style={{ flexDirection: 'column', width: 'match_parent' }}>
          <TextWidget text="Nothing to watch" style={{ fontSize: 18, fontWeight: 'bold', color: c.success }} />
          <TextWidget
            text={
              state.trackedCount === 0
                ? 'No budgeted categories yet'
                : `${state.trackedCount} categories tracking to plan`
            }
            maxLines={2}
            truncate="END"
            style={{ fontSize: 12, color: c.muted, marginTop: 2 }}
          />
        </FlexWidget>
        <FlexWidget style={{ width: 0, height: 0 }} />
      </Shell>
    );
  }

  const overCount = state.pressured.filter((entry) => entry.reason === 'over').length;

  return (
    <Shell>
      <Header
        title="Watch"
        trailing={overCount > 0 ? `${overCount} over` : 'running hot'}
        trailingColor={overCount > 0 ? c.danger : c.warning}
      />
      <FlexWidget style={{ width: 'match_parent', flexDirection: 'column' }}>
        {state.pressured.map((entry) => (
          <Row key={entry.id} entry={entry} />
        ))}
      </FlexWidget>
    </Shell>
  );
}
