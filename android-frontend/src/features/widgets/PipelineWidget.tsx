import { FlexWidget, TextWidget } from 'react-native-android-widget';
import type { PipelineWidgetState } from './pipelineData';
import { shortRelativeTime, widgetColors as c } from './widgetTheme';

/**
 * Home-screen widget for email-to-transaction ingestion health.
 *
 * Rendered by `widgetTaskHandler` into RemoteViews, so this is not a React
 * Native tree: only the library's widget primitives work here, layout is
 * limited to what RemoteViews supports, and there is no state or effects. Keep
 * it small -- the rendered view crosses a Binder boundary with a size limit.
 */

export const PIPELINE_WIDGET_NAME = 'Pipeline';

/** clickAction values handled in the task handler. */
export const RETRY_PARKED_ACTION = 'RETRY_PARKED';

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <FlexWidget
      clickAction="OPEN_APP"
      style={{
        height: 'match_parent',
        width: 'match_parent',
        flexDirection: 'column',
        justifyContent: 'space-between',
        backgroundColor: c.background,
        borderRadius: 24,
        padding: 14
      }}
    >
      {children}
    </FlexWidget>
  );
}

function Header({ trailing, trailingColor }: { trailing?: string; trailingColor?: `#${string}` }) {
  return (
    <FlexWidget
      style={{
        width: 'match_parent',
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}
    >
      <TextWidget
        text="Ingestion"
        style={{ fontSize: 11, fontWeight: '600', letterSpacing: 1, color: c.muted }}
      />
      {trailing ? (
        <TextWidget
          text={trailing}
          style={{ fontSize: 11, fontWeight: '600', color: trailingColor ?? c.muted }}
        />
      ) : (
        <FlexWidget style={{ width: 0, height: 0 }} />
      )}
    </FlexWidget>
  );
}

function Message({ title, body, tone }: { title: string; body: string; tone?: `#${string}` }) {
  return (
    <FlexWidget style={{ flexDirection: 'column', width: 'match_parent' }}>
      <TextWidget text={title} style={{ fontSize: 18, fontWeight: 'bold', color: tone ?? c.text }} />
      <TextWidget
        text={body}
        maxLines={2}
        truncate="END"
        style={{ fontSize: 12, color: c.muted, marginTop: 2 }}
      />
    </FlexWidget>
  );
}

export function PipelineWidget({ state }: { state: PipelineWidgetState }) {
  if (state.kind === 'loading') {
    return (
      <Shell>
        <Header />
        <Message title="Checking…" body="Reading pipeline status" />
        <FlexWidget style={{ width: 0, height: 0 }} />
      </Shell>
    );
  }

  if (state.kind === 'signedOut') {
    return (
      <Shell>
        <Header />
        <Message title="Sign in" body="Open Pennywise to connect this widget" tone={c.primary} />
        <FlexWidget style={{ width: 0, height: 0 }} />
      </Shell>
    );
  }

  if (state.kind === 'error') {
    return (
      <Shell>
        <Header trailing="offline" trailingColor={c.danger} />
        <Message title="Unavailable" body={state.message} tone={c.danger} />
        <FlexWidget style={{ width: 0, height: 0 }} />
      </Shell>
    );
  }

  if (state.kind === 'retrying') {
    return (
      <Shell>
        <Header trailing="retrying" trailingColor={c.primary} />
        <Message
          title={state.count === 1 ? 'Retrying 1 run' : `Retrying ${state.count} runs`}
          body="Signalling the workflows"
          tone={c.primary}
        />
        <FlexWidget style={{ width: 0, height: 0 }} />
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
        <Header trailing="all clear" trailingColor={c.success} />
        <Message title="Nothing parked" body={detail} tone={c.success} />
        <FlexWidget style={{ width: 0, height: 0 }} />
      </Shell>
    );
  }

  return (
    <Shell>
      <Header trailing="needs attention" trailingColor={c.warning} />

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
