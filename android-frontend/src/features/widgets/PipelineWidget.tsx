import { FlexWidget, TextWidget } from 'react-native-android-widget';
import { Header, Message, Shell, Spacer, StatusCard } from './chrome';
import { shortRelativeTime, widgetColors as c } from './widgetTheme';
import type { PipelineWidgetState } from './pipelineData';

/**
 * Home-screen widget for email-to-transaction ingestion health.
 *
 * Rendered by `widgetTaskHandler` into RemoteViews, so this is not a React
 * Native tree: only the library's widget primitives work here, and there is no
 * state or effects. Keep it small -- the rendered view crosses a Binder
 * boundary with a size limit.
 */

export const PIPELINE_WIDGET_NAME = 'Pipeline';

/** clickAction value handled in the task handler. */
export const RETRY_PARKED_ACTION = 'RETRY_PARKED';

export function PipelineWidget({ state }: { state: PipelineWidgetState }) {
  if (state.kind === 'loading' || state.kind === 'signedOut' || state.kind === 'error') {
    return <StatusCard title="Ingestion" state={state} />;
  }

  if (state.kind === 'retrying') {
    return (
      <Shell>
        <Header title="Ingestion" trailing="retrying" trailingColor={c.primary} />
        <Message
          title={state.count === 1 ? 'Retrying 1 run' : `Retrying ${state.count} runs`}
          body="Signalling the workflows"
          tone={c.primary}
        />
        <Spacer />
      </Shell>
    );
  }

  const parkedCount = state.parked.length;

  if (parkedCount === 0) {
    const latest = state.latest;
    const detail = latest
      ? `Last run ${shortRelativeTime(latest.updatedAt)} · ${latest.transactionsCreated} txn${
          latest.transactionsCreated === 1 ? '' : 's'
        }`
      : 'No runs yet';
    return (
      <Shell>
        <Header title="Ingestion" trailing="all clear" trailingColor={c.success} />
        <Message title="Nothing parked" body={detail} tone={c.success} />
        <Spacer />
      </Shell>
    );
  }

  return (
    <Shell>
      <Header title="Ingestion" trailing="needs attention" trailingColor={c.warning} />

      <FlexWidget style={{ flexDirection: 'row', alignItems: 'center', width: 'match_parent' }}>
        <TextWidget text={String(parkedCount)} style={{ fontSize: 34, fontWeight: 'bold', color: c.warning }} />
        <TextWidget
          text={parkedCount === 1 ? 'run parked' : 'runs parked'}
          style={{ fontSize: 13, color: c.muted, marginLeft: 8 }}
        />
      </FlexWidget>

      {/* Tapping this is handled in the task handler; it does not open the app. */}
      <TextWidget
        text={parkedCount === 1 ? 'Retry run' : 'Retry all'}
        clickAction={RETRY_PARKED_ACTION}
        accessibilityLabel={`Retry ${parkedCount} parked pipeline runs`}
        style={{
          fontSize: 13,
          fontWeight: '600',
          color: c.primary,
          textAlign: 'center',
          backgroundColor: c.primaryMuted,
          borderRadius: 999,
          paddingVertical: 7,
          paddingHorizontal: 16
        }}
      />
    </Shell>
  );
}
