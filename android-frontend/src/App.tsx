import 'react-native-gesture-handler';
import { useEffect, useRef } from 'react';
import { Animated, StyleSheet, View } from 'react-native';
import { Provider } from 'react-redux';
import { DarkTheme, NavigationContainer } from '@react-navigation/native';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { SafeAreaProvider, useSafeAreaInsets } from 'react-native-safe-area-context';
import { StatusBar } from 'expo-status-bar';
import { BarChart3, Landmark, LayoutDashboard, ReceiptText, Sparkles, Tags } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from './app/hooks';
import { store } from './app/store';
import { LoadingStateView } from './components/LoadingStateView';
import { colors, spacing } from './theme';
import { apiClient } from './utils/api';
import type { AppTabParamList, AuthStackParamList, RootStackParamList } from './navigation/types';
import { hydrateAuth } from './features/auth/store/authSlice';
import { LoginScreen } from './features/auth/screens/LoginScreen';
import { fetchAllBudgets } from './features/budget/store/budgetSlice';
import { BudgetOnboardingScreen } from './features/budget/screens/BudgetOnboardingScreen';
import { DashboardScreen } from './features/dashboard/screens/DashboardScreen';
import { BudgetScreen } from './features/budget/screens/BudgetScreen';
import { TransactionsScreen } from './features/transactions/screens/TransactionsScreen';
import { PayeesScreen } from './features/payees/screens/PayeesScreen';
import { LoansScreen } from './features/loans/screens/LoansScreen';
import { SettingsScreen } from './features/settings/screens/SettingsScreen';
import { DocumentsScreen } from './features/documents/screens/DocumentsScreen';
import { TagsScreen } from './features/tags/screens/TagsScreen';
import { ActivityScreen } from './features/pipeline/screens/ActivityScreen';
import { WebSocketProvider } from './features/websocket/WebSocketProvider';
import { AgentChat } from './features/agent/components/AgentChat';
import { registerDevicePushToken } from './features/notifications/push';
import { setupLocationSnap } from './features/notifications/locationSnapTask';

// defines the background notification task; must happen at module scope so the
// task exists when Android wakes the app headlessly
setupLocationSnap();

const AuthStack = createNativeStackNavigator<AuthStackParamList>();
const RootStack = createNativeStackNavigator<RootStackParamList>();
const Tab = createBottomTabNavigator<AppTabParamList>();

const navigationTheme = {
  ...DarkTheme,
  colors: {
    ...DarkTheme.colors,
    primary: colors.primary,
    background: colors.background,
    card: colors.surface,
    text: colors.text,
    border: colors.border,
    notification: colors.danger
  }
};

function AuthNavigator() {
  return (
    <AuthStack.Navigator screenOptions={{ headerShown: false }}>
      <AuthStack.Screen name="Login" component={LoginScreen} />
    </AuthStack.Navigator>
  );
}

function iconForRoute(routeName: keyof AppTabParamList, color: string, size: number) {
  switch (routeName) {
    case 'Dashboard':
      return <LayoutDashboard color={color} size={size} />;
    case 'Budget':
      return <BarChart3 color={color} size={size} />;
    case 'Transactions':
      return <ReceiptText color={color} size={size} />;
    case 'Payees':
      return <Tags color={color} size={size} />;
    case 'Loans':
      return <Landmark color={color} size={size} />;
    case 'Penny':
      return <Sparkles color={color} size={size} />;
    default:
      return null;
  }
}

function TabIcon({ routeName, focused }: { routeName: keyof AppTabParamList; focused: boolean }) {
  const progress = useRef(new Animated.Value(focused ? 1 : 0)).current;

  useEffect(() => {
    Animated.spring(progress, {
      toValue: focused ? 1 : 0,
      useNativeDriver: true,
      speed: 20,
      bounciness: 10
    }).start();
  }, [focused, progress]);

  return (
    <View style={styles.tabIcon}>
      <Animated.View
        style={[
          styles.tabIconPill,
          {
            opacity: progress,
            transform: [{ scale: progress.interpolate({ inputRange: [0, 1], outputRange: [0.8, 1] }) }]
          }
        ]}
      />
      {iconForRoute(routeName, focused ? colors.primary : colors.faint, 22)}
    </View>
  );
}

function AppTabs() {
  const insets = useSafeAreaInsets();

  return (
    <Tab.Navigator
      initialRouteName="Dashboard"
      screenOptions={({ route }) => ({
        headerShown: false,
        tabBarShowLabel: false,
        tabBarHideOnKeyboard: true,
        animation: 'shift',
        tabBarActiveTintColor: colors.primary,
        tabBarInactiveTintColor: colors.muted,
        tabBarStyle: [styles.tabBar, { bottom: Math.max(insets.bottom, spacing.md) }],
        tabBarItemStyle: styles.tabBarItem,
        tabBarIcon: ({ focused }) => <TabIcon routeName={route.name as keyof AppTabParamList} focused={focused} />
      })}
    >
      <Tab.Screen name="Dashboard" component={DashboardScreen} />
      <Tab.Screen name="Budget" component={BudgetScreen} />
      <Tab.Screen name="Transactions" component={TransactionsScreen} />
      <Tab.Screen name="Payees" component={PayeesScreen} />
      <Tab.Screen name="Loans" component={LoansScreen} />
      <Tab.Screen name="Penny" component={AgentChat} />
    </Tab.Navigator>
  );
}

function AppNavigator() {
  return (
    <>
      <WebSocketProvider />
      <RootStack.Navigator screenOptions={{ headerShown: false, animation: 'slide_from_right' }}>
        <RootStack.Screen name="Main" component={AppTabs} />
        <RootStack.Screen name="Settings" component={SettingsScreen} />
        <RootStack.Screen name="Documents" component={DocumentsScreen} />
        <RootStack.Screen name="Tags" component={TagsScreen} />
        <RootStack.Screen name="Activity" component={ActivityScreen} />
      </RootStack.Navigator>
    </>
  );
}

const styles = StyleSheet.create({
  tabBar: {
    position: 'absolute',
    // React Navigation's base tab bar style pins the bar with the logical
    // `start`/`end` insets, which win over `left`/`right` in Yoga no matter the
    // style order. Override the same properties so the bar actually floats.
    start: spacing.lg,
    end: spacing.lg,
    height: 64,
    borderTopWidth: 0,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderLight,
    borderRadius: 32,
    backgroundColor: colors.surface,
    paddingHorizontal: spacing.xs,
    paddingTop: spacing.sm,
    paddingBottom: spacing.sm,
    elevation: 24,
    shadowColor: '#000000',
    shadowOffset: { width: 0, height: 10 },
    shadowOpacity: 0.5,
    shadowRadius: 20
  },
  tabBarItem: {
    height: 48,
    borderRadius: 24,
    // The library's own item style is `justifyContent: 'flex-start'` with
    // padding, which pins the icon to the top once labels are hidden.
    justifyContent: 'center',
    alignItems: 'center',
    padding: 0
  },
  tabIcon: {
    width: 44,
    height: 44,
    alignItems: 'center',
    justifyContent: 'center'
  },
  tabIconPill: {
    ...StyleSheet.absoluteFillObject,
    borderRadius: 22,
    backgroundColor: colors.primaryMuted
  }
});

function RootContent() {
  const dispatch = useAppDispatch();
  const auth = useAppSelector((state) => state.auth);
  const budgets = useAppSelector((state) => state.budgets);

  useEffect(() => {
    dispatch(hydrateAuth());
  }, [dispatch]);

  useEffect(() => {
    if (auth.hydrated && auth.isAuthenticated) {
      dispatch(fetchAllBudgets());
      void registerDevicePushToken();
    }
  }, [auth.hydrated, auth.isAuthenticated, dispatch]);

  if (!auth.hydrated) {
    return <LoadingStateView label="Restoring session" />;
  }

  if (!auth.isAuthenticated) {
    return <AuthNavigator />;
  }

  if (budgets.loading === 'pending' && !budgets.selectedBudget) {
    return <LoadingStateView label="Loading budget" />;
  }

  if (!budgets.selectedBudget) {
    return <BudgetOnboardingScreen />;
  }

  return <AppNavigator />;
}

function AppShell() {
  useEffect(() => {
    apiClient
      .probeRoot()
      .then(({ status, body, url }) => {
        console.log(`[api-probe] GET ${url} -> ${status}: ${body}`);
      })
      .catch((error: unknown) => {
        console.log('[api-probe] GET /api failed', error);
      });
  }, []);

  return (
    <SafeAreaProvider>
      <StatusBar style="light" />
      <NavigationContainer theme={navigationTheme}>
        <RootContent />
      </NavigationContainer>
    </SafeAreaProvider>
  );
}

export default function App() {
  return (
    <Provider store={store}>
      <AppShell />
    </Provider>
  );
}
