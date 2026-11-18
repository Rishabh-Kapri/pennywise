import { useEffect, useMemo, useState } from 'react';
import { Pressable, ScrollView, StyleSheet, useWindowDimensions, View } from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import Svg, { Circle, ClipPath, Defs, G, Image as SvgImage, LinearGradient, Path, Rect, Stop } from 'react-native-svg';
import { Eye, EyeOff } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { useAppSelector } from '../../../app/hooks';
import { colors, screenPadding, spacing } from '../../../theme';
import { apiClient } from '../../../utils/api';
import { formatCurrency, formatDayMonth, todayKey } from '../../../utils/date';
import type { StatementImport } from '../../reconcile/types';
import { BudgetAccountType, type Account } from '../types';

// The card holder is drawn in a 300×190 box: the card peeks out of a stitched
// leather pocket. The grain is a baked image (assets/leather.jpg) because
// react-native-svg has no noise or lighting filters.
const VIEW_W = 300;
const VIEW_H = 190;

const POCKET =
  'M0 62 Q0 46 16 46 L66 46 Q80 46 86 58 Q92 70 108 70 L192 70 Q208 70 214 58 Q220 46 234 46 L284 46 Q300 46 300 62 L300 166 Q300 190 276 190 L24 190 Q0 190 0 166Z';
const POCKET_LIP =
  'M0 62 Q0 46 16 46 L66 46 Q80 46 86 58 Q92 70 108 70 L192 70 Q208 70 214 58 Q220 46 234 46 L284 46 Q300 46 300 62';
const STITCH =
  'M7 64 Q7 53 18 53 L64 53 Q76 53 81 63 Q88 77 108 77 L192 77 Q212 77 219 63 Q224 53 236 53 L282 53 Q293 53 293 64 L293 165 Q293 183 275 183 L25 183 Q7 183 7 165Z';
const STITCH_THREAD =
  'M7 63.6 Q7 52.6 18 52.6 L64 52.6 Q76 52.6 81 62.6 Q88 76.6 108 76.6 L192 76.6 Q212 76.6 219 62.6 Q224 52.6 236 52.6 L282 52.6 Q293 52.6 293 63.6 L293 164.6 Q293 182.6 275 182.6 L25 182.6 Q7 182.6 7 164.6Z';

const leather = require('../../../../assets/leather.jpg');

const HIDE_KEY = 'pennywise.cards.hideBalances';

type CardProps = {
  name: string;
  suffix?: string;
  /** account balance; a card in debt is negative */
  balance: number;
  dueDate?: string;
  hidden: boolean;
  onToggleHidden: () => void;
  width: number;
};

export function CardWallet({ name, suffix, balance, dueDate, hidden, onToggleHidden, width }: CardProps) {
  const height = (width * VIEW_H) / VIEW_W;
  const owed = balance < 0;
  const amount = formatCurrency(Math.abs(balance));

  return (
    <View style={{ width, height }} accessible accessibilityLabel={`${name} card, ${owed ? 'owed' : 'credit'} ${hidden ? 'hidden' : amount}`}>
      <Svg width={width} height={height} viewBox={`0 0 ${VIEW_W} ${VIEW_H}`} style={StyleSheet.absoluteFill}>
        <Defs>
          <LinearGradient id="card" x1="0" y1="0" x2="1" y2="1">
            <Stop offset="0" stopColor="#6E6E6E" />
            <Stop offset="0.45" stopColor="#4A4A4A" />
            <Stop offset="0.55" stopColor="#5C5C5C" />
            <Stop offset="1" stopColor="#2F2F2F" />
          </LinearGradient>
          <LinearGradient id="sheen" x1="0" y1="0" x2="1" y2="0">
            <Stop offset="0" stopColor="#FFFFFF" stopOpacity="0" />
            <Stop offset="0.5" stopColor="#FFFFFF" stopOpacity="0.12" />
            <Stop offset="1" stopColor="#FFFFFF" stopOpacity="0" />
          </LinearGradient>
          <LinearGradient id="chip" x1="0" y1="0" x2="1" y2="1">
            <Stop offset="0" stopColor="#E8D9A8" />
            <Stop offset="1" stopColor="#B59A5A" />
          </LinearGradient>
          <ClipPath id="pocket">
            <Path d={POCKET} />
          </ClipPath>
        </Defs>

        {/* the card */}
        <Rect x={14} y={6} width={272} height={150} rx={14} fill="url(#card)" />
        <Path d="M120 6 L190 6 L110 156 L40 156Z" fill="url(#sheen)" />
        <Rect x={26} y={18} width={18} height={14} rx={3} fill="url(#chip)" />
        <Path d="M26 25 H44 M35 18 V32" stroke="#8C7440" strokeWidth={0.6} strokeOpacity={0.6} />

        {/* soft shadow the pocket casts on the card */}
        <G transform="translate(0 -1.5)">
          <Path d={POCKET_LIP} fill="none" stroke="#000000" strokeOpacity={0.12} strokeWidth={9} />
          <Path d={POCKET_LIP} fill="none" stroke="#000000" strokeOpacity={0.16} strokeWidth={5} />
          <Path d={POCKET_LIP} fill="none" stroke="#000000" strokeOpacity={0.2} strokeWidth={2.5} />
        </G>

        {/* the leather pocket */}
        <Path d={POCKET} fill="#1A1919" />
        <SvgImage
          href={leather}
          x={0}
          y={0}
          width={VIEW_W}
          height={VIEW_H}
          preserveAspectRatio="xMidYMid slice"
          clipPath="url(#pocket)"
        />
        <Path d={POCKET_LIP} fill="none" stroke="#FFFFFF" strokeOpacity={0.18} strokeWidth={1.2} />
        <Path d={STITCH} fill="none" stroke="#000000" strokeOpacity={0.55} strokeWidth={2.4} />
        <Path
          d={STITCH_THREAD}
          fill="none"
          stroke="#B3AEA8"
          strokeOpacity={0.85}
          strokeWidth={1.3}
          strokeDasharray="5 3.5"
          strokeLinecap="round"
        />
        <Circle cx={30} cy={66} r={4.2} fill="#0A0A0A" />
        <Circle cx={29.3} cy={65.3} r={2.6} fill="#3A3A3A" />
        <Circle cx={270} cy={66} r={4.2} fill="#0A0A0A" />
        <Circle cx={269.3} cy={65.3} r={2.6} fill="#3A3A3A" />
      </Svg>

      <View style={[styles.cardTop, { top: height * (16 / VIEW_H), left: width * (52 / VIEW_W), right: width * (26 / VIEW_W) }]}>
        <AppText variant="caption" weight="semibold" numberOfLines={1} style={[styles.onCard, styles.grow]}>
          {name}
        </AppText>
        {suffix ? (
          <AppText variant="caption" tabular style={styles.onCardSoft}>
            {`•••• ${suffix}`}
          </AppText>
        ) : null}
      </View>

      <View style={[styles.pocketText, { left: width * (26 / VIEW_W), right: width * (26 / VIEW_W), bottom: height * (18 / VIEW_H) }]}>
        <AppText variant="caption" style={styles.onCardMuted}>
          {owed ? 'Balance owed' : balance > 0 ? 'Credit balance' : 'Nothing owed'}
        </AppText>
        <View style={styles.amountRow}>
          <AppText variant="title" tabular numberOfLines={1} style={[styles.onCard, styles.amount]}>
            {hidden ? '₹ • • • • •' : amount}
          </AppText>
          <Pressable
            onPress={onToggleHidden}
            hitSlop={12}
            accessibilityRole="button"
            accessibilityLabel={hidden ? 'Show card balances' : 'Hide card balances'}
          >
            {hidden ? <EyeOff size={16} color={colors.onHeroMuted} /> : <Eye size={16} color={colors.onHeroMuted} />}
          </Pressable>
          <View style={styles.grow} />
          {dueDate ? (
            <AppText variant="caption" style={styles.onCardMuted}>
              Due {formatDayMonth(dueDate)}
            </AppText>
          ) : null}
        </View>
      </View>
    </View>
  );
}

/** Upcoming due date per card account, from the newest statement that names one. */
function useCardDueDates(cardIds: string[]) {
  const [dueDates, setDueDates] = useState<Record<string, string>>({});
  const key = cardIds.join(',');

  useEffect(() => {
    if (!key) return;
    let cancelled = false;
    apiClient
      .get<StatementImport[]>('statements')
      .then((imports) => {
        if (cancelled) return;
        const today = todayKey();
        const next: Record<string, string> = {};
        for (const imp of imports ?? []) {
          for (const section of imp.summary ?? []) {
            const due = section.dueDate?.slice(0, 10);
            if (!section.accountId || !due || due < today) continue;
            // the nearest upcoming due date wins
            if (!next[section.accountId] || due < next[section.accountId]) next[section.accountId] = due;
          }
        }
        setDueDates(next);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [key]);

  return dueDates;
}

/** Open credit cards as swipeable card holders. Renders nothing without one. */
export function CardWalletRow() {
  const accounts = useAppSelector((state) => state.accounts.budgetAccounts);
  const cards = useMemo(
    () => accounts.filter((account): account is Account & { id: string } => Boolean(account.id) && account.type === BudgetAccountType.CREDIT_CARD && !account.closed && !account.deleted),
    [accounts]
  );
  const dueDates = useCardDueDates(cards.map((card) => card.id));
  const { width: screenWidth } = useWindowDimensions();
  const [hidden, setHidden] = useState(false);
  const [page, setPage] = useState(0);

  useEffect(() => {
    AsyncStorage.getItem(HIDE_KEY)
      .then((raw) => setHidden(raw === '1'))
      .catch(() => undefined);
  }, []);

  const toggleHidden = () =>
    setHidden((current) => {
      void AsyncStorage.setItem(HIDE_KEY, current ? '0' : '1');
      return !current;
    });

  if (!cards.length) return null;

  const fullWidth = screenWidth - screenPadding * 2;
  const single = cards.length === 1;
  // with several cards the next one peeks in from the right
  const cardWidth = single ? fullWidth : fullWidth - spacing.xxl;
  const interval = cardWidth + spacing.md;

  return (
    <View style={styles.row}>
      <View style={styles.head}>
        <AppText variant="heading" style={styles.grow}>
          Your cards
        </AppText>
        {single ? null : (
          <AppText variant="caption" muted tabular>
            {`${page + 1} of ${cards.length}`}
          </AppText>
        )}
      </View>
      {single ? (
        <CardWallet
          name={cards[0].name}
          suffix={cards[0].suffix}
          balance={cards[0].balance ?? 0}
          dueDate={dueDates[cards[0].id]}
          hidden={hidden}
          onToggleHidden={toggleHidden}
          width={cardWidth}
        />
      ) : (
        <>
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            snapToInterval={interval}
            decelerationRate="fast"
            disableIntervalMomentum
            // bleed to the screen edge so the peek isn't clipped by the padding
            style={styles.bleed}
            contentContainerStyle={styles.track}
            onMomentumScrollEnd={(event) => setPage(Math.round(event.nativeEvent.contentOffset.x / interval))}
          >
            {cards.map((card) => (
              <CardWallet
                key={card.id}
                name={card.name}
                suffix={card.suffix}
                balance={card.balance ?? 0}
                dueDate={dueDates[card.id]}
                hidden={hidden}
                onToggleHidden={toggleHidden}
                width={cardWidth}
              />
            ))}
          </ScrollView>
          <View style={styles.dots}>
            {cards.map((card, index) => (
              <View key={card.id} style={[styles.dot, index === page && styles.dotOn]} />
            ))}
          </View>
        </>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    gap: spacing.md
  },
  head: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: spacing.xs,
    marginTop: spacing.xs
  },
  bleed: {
    marginHorizontal: -screenPadding
  },
  track: {
    paddingHorizontal: screenPadding,
    gap: spacing.md
  },
  grow: {
    flex: 1,
    minWidth: 0
  },
  cardTop: {
    position: 'absolute',
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  pocketText: {
    position: 'absolute',
    gap: 2
  },
  amountRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  amount: {
    flexShrink: 1
  },
  onCard: {
    color: colors.onHero
  },
  onCardSoft: {
    color: 'rgba(255, 255, 255, 0.85)',
    letterSpacing: 1
  },
  onCardMuted: {
    color: colors.onHeroMuted
  },
  dots: {
    flexDirection: 'row',
    justifyContent: 'center',
    gap: 6
  },
  dot: {
    width: 6,
    height: 6,
    borderRadius: 3,
    backgroundColor: colors.surfaceTertiary
  },
  dotOn: {
    width: 18,
    backgroundColor: colors.text
  }
});
