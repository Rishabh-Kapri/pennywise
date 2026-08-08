import { FlexWidget, TextWidget } from 'react-native-android-widget';
import { widgetColors as c } from './widgetTheme';

/**
 * Shared layout pieces so every widget reads as the same surface: dark card,
 * small uppercase-ish label top-left, optional status word top-right.
 *
 * These render to RemoteViews, so there is no state, no effects, and no style
 * cascade -- each component spells out what it needs.
 */

/** RemoteViews has no `display: none`; a zero-sized box is how you omit a slot. */
export function Spacer() {
  return <FlexWidget style={{ width: 0, height: 0 }} />;
}

export function Shell({ children }: { children: React.ReactNode }) {
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

export function Header({
  title,
  trailing,
  trailingColor
}: {
  title: string;
  trailing?: string;
  trailingColor?: `#${string}`;
}) {
  return (
    <FlexWidget
      style={{
        width: 'match_parent',
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}
    >
      <TextWidget text={title} style={{ fontSize: 11, fontWeight: '600', letterSpacing: 1, color: c.muted }} />
      {trailing ? (
        <TextWidget text={trailing} style={{ fontSize: 11, fontWeight: '600', color: trailingColor ?? c.muted }} />
      ) : (
        <Spacer />
      )}
    </FlexWidget>
  );
}

export function Message({
  title,
  body,
  tone
}: {
  title: string;
  body: string;
  tone?: `#${string}`;
}) {
  return (
    <FlexWidget style={{ flexDirection: 'column', width: 'match_parent' }}>
      <TextWidget text={title} style={{ fontSize: 18, fontWeight: 'bold', color: tone ?? c.text }} />
      <TextWidget text={body} maxLines={2} truncate="END" style={{ fontSize: 12, color: c.muted, marginTop: 2 }} />
    </FlexWidget>
  );
}

/** The signed-out / error / loading states every widget shares. */
export function StatusCard({
  title,
  state
}: {
  title: string;
  state: { kind: 'loading' } | { kind: 'signedOut' } | { kind: 'error'; message: string };
}) {
  if (state.kind === 'loading') {
    return (
      <Shell>
        <Header title={title} />
        <Message title="Checking…" body="Loading from Pennywise" />
        <Spacer />
      </Shell>
    );
  }
  if (state.kind === 'signedOut') {
    return (
      <Shell>
        <Header title={title} />
        <Message title="Sign in" body="Open Pennywise to connect this widget" tone={c.primary} />
        <Spacer />
      </Shell>
    );
  }
  return (
    <Shell>
      <Header title={title} trailing="offline" trailingColor={c.danger} />
      <Message title="Unavailable" body={state.message} tone={c.danger} />
      <Spacer />
    </Shell>
  );
}
